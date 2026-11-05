package cluster

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// NodeUsage is one node's metrics-server sample. A node the API omitted is
// absent, not a zero filled in for it.
type NodeUsage struct {
	Name        string
	CPUMilli    int64
	MemoryBytes int64
}

// Usage is cluster CPU and memory from metrics-server. Available is false
// when the API is missing. Callers must not invent a number in that case.
type Usage struct {
	Available   bool
	CPUMilli    int64
	MemoryBytes int64
	Nodes       []NodeUsage
}

// Usage reports node metrics. A missing metrics API returns Available false
// and a nil error. Other failures return the error and no number.
func (s *Session) Usage(ctx context.Context) (Usage, error) {
	cfg, err := s.RESTConfig()
	if err != nil {
		return Usage{}, err
	}
	mc, err := metricsclient.NewForConfig(cfg)
	if err != nil {
		return Usage{}, err
	}
	list, err := mc.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		if MetricsAPIMissing(err) {
			return Usage{}, nil
		}
		return Usage{}, err
	}
	var cpu, mem int64
	nodes := make([]NodeUsage, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		nCPU := item.Usage.Cpu().MilliValue()
		nMem := item.Usage.Memory().Value()
		cpu += nCPU
		mem += nMem
		nodes = append(nodes, NodeUsage{Name: item.Name, CPUMilli: nCPU, MemoryBytes: nMem})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	return Usage{Available: true, CPUMilli: cpu, MemoryBytes: mem, Nodes: nodes}, nil
}

// MetricsAPIMissing reports that metrics-server is not installed or not ready.
func MetricsAPIMissing(err error) bool {
	return apierrors.IsNotFound(err) || apierrors.IsServiceUnavailable(err)
}

// NodeReady is true when the Ready condition is True.
func NodeReady(n *corev1.Node) bool {
	if n == nil {
		return false
	}
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// PodRunning is true when the pod phase is Running.
func PodRunning(p *corev1.Pod) bool {
	return p != nil && p.Status.Phase == corev1.PodRunning
}

// EventWarning is true for Warning events.
func EventWarning(e *corev1.Event) bool {
	return e != nil && e.Type == corev1.EventTypeWarning
}
