package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/streaming/pkg/httpstream"
)

// ExecConn is the terminal connection. Close cancels the remote command.
type ExecConn struct {
	mu        sync.Mutex
	stdin     *io.PipeWriter
	stdout    *io.PipeReader
	stdoutW   *io.PipeWriter
	queue     *sizeQueue
	cancel    context.CancelFunc
	closeOnce sync.Once
	cols      int
	rows      int
	done      chan struct{}
	err       error
}

// Exec starts a remote command. An empty argv is rejected; there is no
// /bin/sh default. The returned conn is an io.ReadWriteCloser with Resize.
func (s *Session) Exec(ctx context.Context, namespace, pod, container string, command []string) (*ExecConn, error) {
	if len(command) == 0 || command[0] == "" {
		return nil, errors.New("exec command is required")
	}
	s.mu.Lock()
	cs := s.cs
	cfg := s.cfg
	s.mu.Unlock()
	if cs == nil || cfg == nil {
		return nil, ErrNoContext
	}
	restClient := cs.CoreV1().RESTClient()
	if restClient == nil {
		return nil, errors.New("exec transport is unavailable")
	}
	req := restClient.Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	streamCtx, cancel := context.WithCancel(ctx)
	conn, stdinR, stdoutW := newExecConn(cancel)
	streamCfg := streamingConfig(streamCtx, cfg)

	wsExec, err := remotecommand.NewWebSocketExecutor(streamCfg, http.MethodGet, req.URL().String())
	if err != nil {
		cancel()
		return nil, err
	}
	spdyExec, err := remotecommand.NewSPDYExecutor(streamCfg, http.MethodPost, req.URL())
	if err != nil {
		cancel()
		return nil, err
	}
	executor, err := remotecommand.NewFallbackExecutor(wsExec, spdyExec, func(err error) bool {
		return shouldFallbackExec(streamCtx, err)
	})
	if err != nil {
		cancel()
		return nil, err
	}

	go func() {
		defer close(conn.done)
		defer stdoutW.Close()
		err := executor.StreamWithContext(streamCtx, remotecommand.StreamOptions{
			Stdin:             stdinR,
			Stdout:            stdoutW,
			Stderr:            stdoutW,
			Tty:               true,
			TerminalSizeQueue: conn.queue,
		})
		conn.mu.Lock()
		conn.err = err
		conn.mu.Unlock()
		if err != nil && streamCtx.Err() == nil {
			msg := "\r\nexec: " + err.Error() + "\r\n"
			_, _ = stdoutW.Write([]byte(msg))
		}
	}()
	return conn, nil
}

func newExecConn(cancel context.CancelFunc) (*ExecConn, *io.PipeReader, *io.PipeWriter) {
	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	if cancel == nil {
		cancel = func() {}
	}
	conn := &ExecConn{
		stdin:   stdinW,
		stdout:  stdoutR,
		stdoutW: stdoutW,
		queue:   newSizeQueue(),
		cancel:  cancel,
		done:    make(chan struct{}),
	}
	// stdinR is the remote's read side. The caller of newExecConn owns it.
	_ = stdinR
	return conn, stdinR, stdoutW
}

// Read reads remote stdout.
func (c *ExecConn) Read(p []byte) (int, error) {
	if c.stdout == nil {
		return 0, io.EOF
	}
	return c.stdout.Read(p)
}

// Write sends stdin to the remote command.
func (c *ExecConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	w := c.stdin
	c.mu.Unlock()
	if w == nil {
		return 0, io.ErrClosedPipe
	}
	return w.Write(p)
}

// Resize records the terminal size and forwards it to the remote command.
func (c *ExecConn) Resize(cols, rows int) error {
	if cols < 0 || rows < 0 || cols > 65535 || rows > 65535 {
		return fmt.Errorf("invalid terminal size %d×%d", cols, rows)
	}
	c.mu.Lock()
	c.cols, c.rows = cols, rows
	q := c.queue
	c.mu.Unlock()
	if q != nil {
		q.push(uint16(cols), uint16(rows))
	}
	return nil
}

// Size is the last size Resize recorded.
func (c *ExecConn) Size() (cols, rows int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cols, c.rows
}

// Close cancels the remote command and closes both pipes.
func (c *ExecConn) Close() error {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		c.mu.Lock()
		w := c.stdin
		q := c.queue
		stdoutW := c.stdoutW
		c.stdin = nil
		c.mu.Unlock()
		if w != nil {
			_ = w.Close()
		}
		if stdoutW != nil {
			_ = stdoutW.Close()
		}
		if q != nil {
			q.close()
		}
	})
	return nil
}

type sizeQueue struct {
	mu     sync.Mutex
	ch     chan remotecommand.TerminalSize
	closed bool
}

func newSizeQueue() *sizeQueue {
	return &sizeQueue{ch: make(chan remotecommand.TerminalSize, 4)}
}

func (q *sizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.ch
	if !ok {
		return nil
	}
	cp := size
	return &cp
}

func (q *sizeQueue) push(width, height uint16) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	size := remotecommand.TerminalSize{Width: width, Height: height}
	select {
	case q.ch <- size:
	default:
		select {
		case <-q.ch:
		default:
		}
		select {
		case q.ch <- size:
		default:
		}
	}
}

func (q *sizeQueue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	close(q.ch)
}

func streamingConfig(ctx context.Context, cfg *rest.Config) *rest.Config {
	copy := rest.CopyConfig(cfg)
	copy.Timeout = 0
	copy.NextProtos = []string{"http/1.1"}
	base := cfg.Dial
	if base == nil {
		d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		base = d.DialContext
	}
	copy.Dial = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return base(dialCtx, network, address)
	}
	return copy
}

func shouldFallbackExec(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if apierrors.IsUnauthorized(err) {
		return false
	}
	if apierrors.IsForbidden(err) {
		return true
	}
	var upgrade *httpstream.UpgradeFailureError
	if errors.As(err, &upgrade) {
		if upgrade.Cause != nil && apierrors.IsUnauthorized(upgrade.Cause) {
			return false
		}
		return true
	}
	return false
}
