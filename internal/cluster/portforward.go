package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwardStartupTimeout bounds dialing, authorization, and local listener
// setup even when a caller supplies context.Background.
const PortForwardStartupTimeout = 20 * time.Second

const portForwardCleanupWait = 100 * time.Millisecond

// Forward is one port-forward this process started. It binds localhost only.
type Forward struct {
	Context   string
	Namespace string
	Pod       string
	Local     uint16
	Remote    uint16

	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	err      error
	stopping bool
}

// Stop asks the forwarding process to close. It is safe to call repeatedly.
// Errors produced after an intentional stop are normal shutdown, not failure.
func (f *Forward) Stop() {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.stopping = true
	f.mu.Unlock()
	if f.stop != nil {
		f.stopOnce.Do(func() { close(f.stop) })
	}
}

// Done closes after the forwarding process has released its listener.
func (f *Forward) Done() <-chan struct{} {
	if f == nil {
		return closedForwardDone
	}
	return f.done
}

// Err reports an unexpected forwarding-process exit.
func (f *Forward) Err() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// Status is suitable for the process-owned forwards list.
func (f *Forward) Status() string {
	if f == nil {
		return "Stopped"
	}
	select {
	case <-f.done:
		if f.Err() != nil {
			return "Failed"
		}
		return "Stopped"
	default:
	}
	f.mu.Lock()
	stopping := f.stopping
	f.mu.Unlock()
	if stopping {
		return "Stopping"
	}
	return "Active"
}

var closedForwardDone = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

func (f *Forward) finish(err error, stderr *bytes.Buffer) {
	f.mu.Lock()
	if f.stopping {
		err = nil
	} else if err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" && !strings.Contains(err.Error(), detail) {
			err = fmt.Errorf("%w: %s", err, detail)
		}
	}
	f.err = err
	f.mu.Unlock()
	close(f.done)
}

// PortForward starts a forward bound to the currently selected context.
func (s *Session) PortForward(ctx context.Context, namespace, pod string, local, remote uint16) (*Forward, error) {
	return s.PortForwardInContext(ctx, s.Current(), namespace, pod, local, remote)
}

// PortForwardInContext starts a forward only if expectedContext is still
// selected. The client, REST config, and context identity are captured
// atomically so a concurrent context switch cannot retarget the request.
func (s *Session) PortForwardInContext(ctx context.Context, expectedContext, namespace, pod string, local, remote uint16) (*Forward, error) {
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("port-forward requires a namespace")
	}
	if strings.TrimSpace(pod) == "" {
		return nil, errors.New("port-forward requires a pod")
	}
	if local == 0 {
		return nil, errors.New("local port must be between 1 and 65535")
	}
	if remote == 0 {
		return nil, errors.New("remote port must be between 1 and 65535")
	}
	cs, cfg, err := s.portForwardClients(expectedContext)
	if err != nil {
		return nil, err
	}
	req := cs.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward").
		URL()
	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("configure port-forward transport: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req)
	stop := make(chan struct{})
	ready := make(chan struct{})
	var stderr bytes.Buffer
	pf, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, []string{fmt.Sprintf("%d:%d", local, remote)}, stop, ready, io.Discard, &stderr)
	if err != nil {
		return nil, fmt.Errorf("configure port-forward: %w", err)
	}
	fw := &Forward{
		Context: expectedContext, Namespace: namespace, Pod: pod,
		Local: local, Remote: remote, stop: stop, done: make(chan struct{}),
	}
	startupCtx, cancel := context.WithTimeout(ctx, PortForwardStartupTimeout)
	defer cancel()
	if err := awaitForwardReady(startupCtx, fw, ready, &stderr, pf.ForwardPorts); err != nil {
		return fw, err
	}
	return fw, nil
}

func awaitForwardReady(ctx context.Context, fw *Forward, ready <-chan struct{}, stderr *bytes.Buffer, run func() error) error {
	go func() {
		fw.finish(run(), stderr)
	}()
	select {
	case <-ready:
		select {
		case <-fw.done:
			return forwardStartupError(fw.Err())
		default:
			return nil
		}
	case <-fw.done:
		return forwardStartupError(fw.Err())
	case <-ctx.Done():
		fw.Stop()
		select {
		case <-fw.done:
		case <-time.After(portForwardCleanupWait):
		}
		return ctx.Err()
	}
}

func forwardStartupError(err error) error {
	if err == nil {
		return errors.New("port-forward stopped before becoming ready")
	}
	return fmt.Errorf("start port-forward: %w", err)
}

func (s *Session) portForwardClients(expectedContext string) (kubernetes.Interface, *rest.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.name != expectedContext {
		return nil, nil, errors.New("kube context changed before the port-forward started")
	}
	if s.cs == nil || s.cfg == nil {
		return nil, nil, ErrNoContext
	}
	return s.cs, rest.CopyConfig(s.cfg), nil
}

// ReadyPod lists EndpointSlices and returns one ready pod for the service.
func (s *Session) ReadyPod(ctx context.Context, namespace, service string) (string, error) {
	return s.ReadyPodInContext(ctx, s.Current(), namespace, service)
}

// ReadyPodInContext resolves the service only if expectedContext is still
// selected, preventing a service action from crossing a context switch.
func (s *Session) ReadyPodInContext(ctx context.Context, expectedContext, namespace, service string) (string, error) {
	s.mu.Lock()
	if s.name != expectedContext {
		s.mu.Unlock()
		return "", errors.New("kube context changed before the port-forward started")
	}
	cs := s.cs
	s.mu.Unlock()
	if cs == nil {
		return "", ErrNoContext
	}
	selector := labels.Set{"kubernetes.io/service-name": service}.AsSelector().String()
	list, err := cs.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", err
	}
	return ReadyPodForService(list.Items, service)
}

// ReadyPodForService returns one ready endpoint pod. It refuses when none is ready.
// A nil Ready condition means ready.
func ReadyPodForService(slices []discoveryv1.EndpointSlice, service string) (string, error) {
	for i := range slices {
		slice := &slices[i]
		if slice.Labels["kubernetes.io/service-name"] != service {
			continue
		}
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" && ep.TargetRef.Name != "" {
				return ep.TargetRef.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no ready pod for service %s", service)
}
