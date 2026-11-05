package cluster

import (
	"context"
	"fmt"
	"io"
	"net/http"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Forward is one port-forward this process started. It binds localhost only.
type Forward struct {
	Namespace string
	Pod       string
	Local     uint16
	Remote    uint16
	stop      chan struct{}
}

// Stop closes the forward.
func (f *Forward) Stop() {
	if f == nil || f.stop == nil {
		return
	}
	select {
	case <-f.stop:
	default:
		close(f.stop)
	}
}

// PortForward dials pods/{name}/portforward and binds 127.0.0.1 only.
func (s *Session) PortForward(ctx context.Context, namespace, pod string, local, remote uint16) (*Forward, error) {
	if pod == "" {
		return nil, fmt.Errorf("port-forward requires a pod")
	}
	cs, err := s.clientset()
	if err != nil {
		return nil, err
	}
	cfg, err := s.RESTConfig()
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
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req)
	stop := make(chan struct{})
	ready := make(chan struct{})
	pf, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, []string{fmt.Sprintf("%d:%d", local, remote)}, stop, ready, io.Discard, io.Discard)
	if err != nil {
		return nil, err
	}
	go func() { _ = pf.ForwardPorts() }()
	select {
	case <-ready:
	case <-ctx.Done():
		close(stop)
		return nil, ctx.Err()
	}
	return &Forward{Namespace: namespace, Pod: pod, Local: local, Remote: remote, stop: stop}, nil
}

// ReadyPod lists EndpointSlices and returns one ready pod for the service.
func (s *Session) ReadyPod(ctx context.Context, namespace, service string) (string, error) {
	cs, err := s.clientset()
	if err != nil {
		return "", err
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
