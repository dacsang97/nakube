package cluster

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestMetricsAPIMissingNotFound(t *testing.T) {
	err := apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "nodes"}, "nodes")
	if !MetricsAPIMissing(err) {
		t.Fatal("NotFound should mean metrics are unavailable")
	}
	if MetricsAPIMissing(apierrors.NewUnauthorized("no")) {
		t.Fatal("unauthorized is not a missing API")
	}
}

func TestPodUsagesSumContainers(t *testing.T) {
	use := func(cpu, mem string) corev1.ResourceList {
		return corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem)}
	}
	got := podUsages(&metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "web"},
		Containers: []metricsv1beta1.ContainerMetrics{
			{Name: "app", Usage: use("250m", "64Mi")},
			{Name: "proxy", Usage: use("50m", "16Mi")},
		},
	}}})
	if u, ok := got["shop/web"]; !ok || u.CPUMilli != 300 || u.MemoryBytes != 80*1024*1024 {
		t.Fatalf("got %+v", got)
	}
	if _, ok := got["web"]; ok {
		t.Fatal("key lost its namespace")
	}
}

func TestPodUsagesFallBackPerNamespace(t *testing.T) {
	// The fake's tracker does not serve PodMetrics lists, so the reactor
	// answers for each namespace.
	mc := metricsfake.NewSimpleClientset()
	mc.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if ns := action.GetNamespace(); ns == "" || ns == "ops" {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("no"))
		}
		return true, &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{
			{ObjectMeta: metav1.ObjectMeta{Namespace: action.GetNamespace(), Name: "web"}},
		}}, nil
	})
	got, ok, err := readPodUsages(context.Background(), mc, []string{"ops", "shop"})
	if err != nil || !ok {
		t.Fatalf("ok %v err %v", ok, err)
	}
	if _, has := got["shop/web"]; !has || len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if _, _, err := readPodUsages(context.Background(), mc, nil); !apierrors.IsForbidden(err) {
		t.Fatalf("no fallback must report the denial, got %v", err)
	}
}
