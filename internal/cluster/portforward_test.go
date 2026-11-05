package cluster

import (
	"testing"

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
