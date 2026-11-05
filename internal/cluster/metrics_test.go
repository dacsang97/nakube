package cluster

import (
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
