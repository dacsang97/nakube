package cluster

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReadyPodForService(t *testing.T) {
	ready := true
	notReady := false
	slices := []discoveryv1.EndpointSlice{{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"kubernetes.io/service-name": "web"}},
		Endpoints: []discoveryv1.Endpoint{
			{Conditions: discoveryv1.EndpointConditions{Ready: &notReady}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "web-0"}},
			{Conditions: discoveryv1.EndpointConditions{Ready: &ready}, TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "web-1"}},
		},
	}}
	name, err := ReadyPodForService(slices, "web")
	if err != nil || name != "web-1" {
		t.Fatalf("pod=%q err=%v", name, err)
	}
	if _, err := ReadyPodForService(slices, "missing"); err == nil {
		t.Fatal("missing service was forwarded")
	}
}

func TestAwaitForwardReadyReturnsEarlyProcessFailure(t *testing.T) {
	stop := make(chan struct{})
	fw := &Forward{stop: stop, done: make(chan struct{})}
	ready := make(chan struct{})
	var stderr bytes.Buffer
	err := awaitForwardReady(context.Background(), fw, ready, &stderr, func() error {
		stderr.WriteString("unable to listen on 127.0.0.1:8080")
		return errors.New("bind failed")
	})
	if err == nil || !strings.Contains(err.Error(), "bind failed") || !strings.Contains(err.Error(), "unable to listen") {
		t.Fatalf("startup error = %v", err)
	}
	select {
	case <-fw.Done():
	default:
		t.Fatal("failed process did not finish")
	}
}

func TestForwardStopWaitsForProcessExit(t *testing.T) {
	stop := make(chan struct{})
	release := make(chan struct{})
	fw := &Forward{stop: stop, done: make(chan struct{})}
	ready := make(chan struct{})
	if err := awaitForwardReady(context.Background(), fw, ready, &bytes.Buffer{}, func() error {
		close(ready)
		<-stop
		<-release
		return errors.New("use of closed network connection")
	}); err != nil {
		t.Fatal(err)
	}
	if got := fw.Status(); got != "Active" {
		t.Fatalf("status before stop = %q", got)
	}
	fw.Stop()
	if got := fw.Status(); got != "Stopping" {
		t.Fatalf("status while stopping = %q", got)
	}
	close(release)
	select {
	case <-fw.Done():
	case <-time.After(time.Second):
		t.Fatal("forward process did not stop")
	}
	if got := fw.Status(); got != "Stopped" {
		t.Fatalf("status after stop = %q", got)
	}
}

func TestAwaitForwardReadyCancelsBlockedStartup(t *testing.T) {
	stop := make(chan struct{})
	fw := &Forward{stop: stop, done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := awaitForwardReady(ctx, fw, make(chan struct{}), &bytes.Buffer{}, func() error {
		<-stop
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	select {
	case <-fw.Done():
	case <-time.After(time.Second):
		t.Fatal("cancelled startup process did not exit")
	}
}

func TestCancelledStartupReturnsWithOwnedForwardStillStopping(t *testing.T) {
	stop := make(chan struct{})
	release := make(chan struct{})
	fw := &Forward{stop: stop, done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	err := awaitForwardReady(ctx, fw, make(chan struct{}), &bytes.Buffer{}, func() error {
		<-stop
		<-release
		return errors.New("late shutdown error")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation blocked on delayed shutdown")
	}
	select {
	case <-fw.Done():
		t.Fatal("forward ownership ended before delayed worker released")
	default:
	}
	if fw.Status() != "Stopping" {
		t.Fatalf("delayed status = %q", fw.Status())
	}
	close(release)
	select {
	case <-fw.Done():
	case <-time.After(time.Second):
		t.Fatal("owned forward did not finish")
	}
	if fw.Err() != nil || fw.Status() != "Stopped" {
		t.Fatalf("intentional shutdown err=%v status=%q", fw.Err(), fw.Status())
	}
}

func TestPortForwardValidatesInputsBeforeConnecting(t *testing.T) {
	s := NewSession()
	tests := []struct {
		namespace string
		pod       string
		local     uint16
		remote    uint16
		want      string
	}{
		{"", "web", 8080, 80, "namespace"},
		{"default", "", 8080, 80, "pod"},
		{"default", "web", 0, 80, "local port"},
		{"default", "web", 8080, 0, "remote port"},
	}
	for _, tt := range tests {
		_, err := s.PortForwardInContext(context.Background(), "dev", tt.namespace, tt.pod, tt.local, tt.remote)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("PortForwardInContext(%q, %q, %d, %d) error = %v", tt.namespace, tt.pod, tt.local, tt.remote, err)
		}
	}
}
