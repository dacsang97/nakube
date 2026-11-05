package desktop

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/dacsang97/nakube/internal/cluster"
)

// Overview is the numbers the first screen draws. MetricsText is never an
// invented figure: a missing API is the words "metrics unavailable".
type Overview struct {
	NodesReady  int
	NodesTotal  int
	PodsRunning int
	PodsTotal   int
	Warnings    int
	CPU         string
	Memory      string
}

func (o Overview) NodesText() string {
	return fmt.Sprintf("%d/%d", o.NodesReady, o.NodesTotal)
}

func (o Overview) PodsText() string {
	return fmt.Sprintf("%d/%d", o.PodsRunning, o.PodsTotal)
}

func (o Overview) WarningsText() string {
	return fmt.Sprintf("%d", o.Warnings)
}

// metricsCardText is what the CPU and memory cards show. NotFound and
// ServiceUnavailable say the API is missing. A real value is shown only
// when the metrics client returned one.
func metricsCardText(err error, available bool, value string) string {
	if apierrors.IsNotFound(err) || apierrors.IsServiceUnavailable(err) || !available {
		return "metrics unavailable"
	}
	if value == "" {
		return "metrics unavailable"
	}
	return value
}

func formatCPU(milli int64) string {
	return fmt.Sprintf("%dm", milli)
}

func formatBytes(n int64) string {
	const (
		ki = 1024
		mi = 1024 * ki
		gi = 1024 * mi
	)
	switch {
	case n >= gi:
		return fmt.Sprintf("%.1f Gi", float64(n)/float64(gi))
	case n >= mi:
		return fmt.Sprintf("%.1f Mi", float64(n)/float64(mi))
	case n >= ki:
		return fmt.Sprintf("%.1f Ki", float64(n)/float64(ki))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func overviewFrom(nodes map[string]*corev1.Node, pods map[string]*corev1.Pod, events map[string]*corev1.Event, usage cluster.Usage, usageErr error) Overview {
	o := Overview{
		CPU:    metricsCardText(usageErr, usage.Available, formatCPU(usage.CPUMilli)),
		Memory: metricsCardText(usageErr, usage.Available, formatBytes(usage.MemoryBytes)),
	}
	for _, n := range nodes {
		o.NodesTotal++
		if cluster.NodeReady(n) {
			o.NodesReady++
		}
	}
	for _, p := range pods {
		o.PodsTotal++
		if cluster.PodRunning(p) {
			o.PodsRunning++
		}
	}
	for _, e := range events {
		if cluster.EventWarning(e) {
			o.Warnings++
		}
	}
	return o
}
