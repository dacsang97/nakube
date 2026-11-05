package cluster

import (
	"bufio"
	"context"
	"io"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	logScannerMaxLine = 1024 * 1024
	logFlushInterval  = 60 * time.Millisecond
	logFlushMaxLines  = 256
)

// LogLine is one owned line from a fan-out stream.
type LogLine struct {
	Pod       string
	Container string
	Text      string
}

// StreamLogs follows one container. Each batch passed to onBatch is a copy.
func (s *Session) StreamLogs(ctx context.Context, namespace, pod, container string, follow bool, tail int64, onBatch func([]string)) error {
	cs, err := s.clientset()
	if err != nil {
		return err
	}
	opts := &corev1.PodLogOptions{Container: container, Follow: follow}
	if tail > 0 {
		opts.TailLines = &tail
	}
	stream, err := cs.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()
	return scanLogStream(ctx, stream, onBatch)
}

// FanoutLogs lists pods by selector and starts one stream per container.
// Lines are tagged with pod and container. Each batch is copied before return.
func (s *Session) FanoutLogs(ctx context.Context, namespace, selector string, follow bool, tail int64, onBatch func([]LogLine)) error {
	cs, err := s.clientset()
	if err != nil {
		return err
	}
	list, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	targets := fanoutTargets(list.Items)
	if len(targets) == 0 {
		return nil
	}
	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(podName, container string) {
			defer wg.Done()
			_ = s.StreamLogs(ctx, namespace, podName, container, follow, tail, func(lines []string) {
				if onBatch == nil {
					return
				}
				onBatch(tagLines(podName, container, lines))
			})
		}(target.pod, target.container)
	}
	wg.Wait()
	return nil
}

type logTarget struct {
	pod       string
	container string
}

func fanoutTargets(pods []corev1.Pod) []logTarget {
	var out []logTarget
	for i := range pods {
		pod := &pods[i]
		for _, container := range pod.Spec.Containers {
			out = append(out, logTarget{pod: pod.Name, container: container.Name})
		}
	}
	return out
}

func tagLines(pod, container string, lines []string) []LogLine {
	owned := ownLines(lines)
	out := make([]LogLine, len(owned))
	for i, text := range owned {
		out[i] = LogLine{Pod: pod, Container: container, Text: text}
	}
	return out
}

func ownLines(lines []string) []string {
	if lines == nil {
		return nil
	}
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

func deliverLines(lines []string, onBatch func([]string)) {
	if onBatch == nil || len(lines) == 0 {
		return
	}
	onBatch(ownLines(lines))
}

func scanLogStream(ctx context.Context, r io.Reader, onBatch func([]string)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), logScannerMaxLine)
	batch := make([]string, 0, 64)
	flush := func() {
		deliverLines(batch, onBatch)
		batch = batch[:0]
	}

	lines := make(chan string, logFlushMaxLines)
	errCh := make(chan error, 1)
	go func() {
		defer close(lines)
		for sc.Scan() {
			select {
			case <-ctx.Done():
				return
			case lines <- sc.Text():
			}
		}
		if err := sc.Err(); err != nil && ctx.Err() == nil {
			errCh <- err
		}
	}()

	ticker := time.NewTicker(logFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-lines:
			if !ok {
				if ctx.Err() == nil {
					flush()
				}
				select {
				case err := <-errCh:
					return err
				default:
					return nil
				}
			}
			batch = append(batch, line)
			if len(batch) >= logFlushMaxLines {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
