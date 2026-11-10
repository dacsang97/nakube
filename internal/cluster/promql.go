package cluster

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Metric tabs. Each one is a chart in a resource's Metrics block.
const (
	MetricCPU        = "cpu"
	MetricMemory     = "memory"
	MetricNetwork    = "network"
	MetricFilesystem = "filesystem"
)

// MetricTabs are the tabs in the order the toolbar shows them.
func MetricTabs() []string {
	return []string{MetricCPU, MetricMemory, MetricNetwork, MetricFilesystem}
}

// MetricQuery is one series of a chart. Line false is the primary series,
// drawn as bars. Line true is drawn over them.
type MetricQuery struct {
	Name string
	Expr string
	Line bool
}

// PodQueries are the series for pods in one namespace, the queries the Lens
// operator provider runs: cAdvisor for usage, kube-state-metrics for
// requests and limits. An empty container is every container in the pods.
// No pods is no query, not a query that matches everything.
func PodQueries(tab, namespace string, pods []string, container string, step time.Duration) []MetricQuery {
	if len(pods) == 0 || namespace == "" {
		return nil
	}
	sel := "namespace=" + strconv.Quote(namespace) + ",pod=~" + strconv.Quote(podPattern(pods))
	cont := `container!="",container!="POD"`
	ksm := sel
	if container != "" {
		cont = "container=" + strconv.Quote(container)
		ksm += "," + cont
	}
	cad := sel + "," + cont + `,image!=""`
	r := RateWindow(step)
	switch tab {
	case MetricCPU:
		return []MetricQuery{
			{Name: "Usage", Expr: fmt.Sprintf("sum(rate(container_cpu_usage_seconds_total{%s}[%s]))", cad, r)},
			{Name: "Requests", Expr: fmt.Sprintf(`sum(kube_pod_container_resource_requests{%s,resource="cpu"})`, ksm), Line: true},
			{Name: "Limits", Expr: fmt.Sprintf(`sum(kube_pod_container_resource_limits{%s,resource="cpu"})`, ksm), Line: true},
		}
	case MetricMemory:
		return []MetricQuery{
			{Name: "Usage", Expr: fmt.Sprintf("sum(container_memory_working_set_bytes{%s})", cad)},
			{Name: "Requests", Expr: fmt.Sprintf(`sum(kube_pod_container_resource_requests{%s,resource="memory"})`, ksm), Line: true},
			{Name: "Limits", Expr: fmt.Sprintf(`sum(kube_pod_container_resource_limits{%s,resource="memory"})`, ksm), Line: true},
		}
	case MetricNetwork:
		// Network is counted on the pod's sandbox, so a container filter
		// would match nothing.
		return []MetricQuery{
			{Name: "Receive", Expr: fmt.Sprintf("sum(rate(container_network_receive_bytes_total{%s}[%s]))", sel, r)},
			{Name: "Transmit", Expr: fmt.Sprintf("sum(rate(container_network_transmit_bytes_total{%s}[%s]))", sel, r), Line: true},
		}
	case MetricFilesystem:
		return []MetricQuery{
			{Name: "Usage", Expr: fmt.Sprintf("sum(container_fs_usage_bytes{%s})", cad)},
			{Name: "Reads", Expr: fmt.Sprintf("sum(rate(container_fs_reads_bytes_total{%s}[%s]))", cad, r), Line: true},
			{Name: "Writes", Expr: fmt.Sprintf("sum(rate(container_fs_writes_bytes_total{%s}[%s]))", cad, r), Line: true},
		}
	default:
		return nil
	}
}

// podPattern matches exactly the named pods, anchored, with each name
// quoted so a dot cannot match another character.
func podPattern(pods []string) string {
	quoted := make([]string, len(pods))
	for i, p := range pods {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

// MetricStep keeps a range near 60 slots, as Lens does, and never under
// a minute: 1h is 60s, 6h is 6m, 24h is 24m, 7d is 168m.
func MetricStep(rng time.Duration) time.Duration {
	step := (rng / 60).Round(time.Second)
	if step < time.Minute {
		return time.Minute
	}
	return step
}

// RateWindow is twice the step, and at least a minute, so a rate always
// spans two scrapes.
func RateWindow(step time.Duration) string {
	w := 2 * step
	if w < time.Minute {
		w = time.Minute
	}
	return strconv.Itoa(int(w/time.Second)) + "s"
}
