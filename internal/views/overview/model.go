package overview

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/dacsang97/nakube/internal/cluster"
)

const (
	Worker = "worker"
	Master = "master"
	CPU    = "cpu"
	Memory = "memory"
)

// Window is one pill in the time control. Samples outside it are not drawn.
type Window struct {
	Label string
	Dur   time.Duration
}

// Windows are the time ranges, plus a day for a cluster that has been open.
func Windows() []Window {
	return []Window{
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"1h", time.Hour},
		{"24h", 24 * time.Hour},
	}
}

// WindowBy returns the named window, or 15m when the name is unknown.
func WindowBy(label string) Window {
	for _, w := range Windows() {
		if w.Label == label {
			return w
		}
	}
	return Windows()[2]
}

// Sample is one metrics-server observation. Nodes are the API's list, copied.
type Sample struct {
	At    time.Time
	Nodes []cluster.NodeUsage
}

// Trim drops samples older than the longest window, plus a little slack.
func Trim(samples []Sample, now time.Time) []Sample {
	cut := now.Add(-25 * time.Hour)
	i := 0
	for i < len(samples) && samples[i].At.Before(cut) {
		i++
	}
	if i == 0 {
		return samples
	}
	return append([]Sample(nil), samples[i:]...)
}

// Snapshot is the cluster-wide reading the phone and the screen share.
// CPU and Memory are never an invented figure.
type Snapshot struct {
	NodesReady  int
	NodesTotal  int
	PodsRunning int
	PodsTotal   int
	Warnings    int
	CPU         string
	Memory      string
	MetricsOK   bool
	NotReady    []string
	Nodes       []nodeFact
	Pods        []podFact
	Events      []Event
}

// Event is one warning, newest first when listed.
type Event struct {
	When    time.Time
	Object  string
	Reason  string
	Message string
}

func (s Snapshot) NodesText() string {
	return fmt.Sprintf("%d/%d", s.NodesReady, s.NodesTotal)
}

func (s Snapshot) PodsText() string {
	return fmt.Sprintf("%d/%d", s.PodsRunning, s.PodsTotal)
}

func (s Snapshot) WarningsText() string {
	return fmt.Sprintf("%d", s.Warnings)
}

type nodeFact struct {
	Name             string
	Master           bool
	Ready            bool
	CPUAlloc, CPUCap int64
	MemAlloc, MemCap int64
	PodAlloc, PodCap int64
}

type podFact struct {
	Node                 string
	Phase                corev1.PodPhase
	CPURequest, CPULimit int64
	MemRequest, MemLimit int64
}

// From counts nodes, pods, and warnings. A missing metrics API leaves CPU
// and Memory as "metrics unavailable" and does not store a usage number.
func From(nodes map[string]*corev1.Node, pods map[string]*corev1.Pod, events map[string]*corev1.Event, usage cluster.Usage, usageErr error, now time.Time) Snapshot {
	s := Snapshot{
		CPU:       metricsText(usageErr, usage.Available, formatMilli(usage.CPUMilli)),
		Memory:    metricsText(usageErr, usage.Available, formatBytes(usage.MemoryBytes)),
		MetricsOK: usage.Available && usageErr == nil,
	}
	var notReady []string
	for _, n := range nodes {
		if n == nil {
			continue
		}
		s.NodesTotal++
		fact := nodeFact{
			Name:     n.Name,
			Master:   masterNode(n),
			Ready:    cluster.NodeReady(n),
			CPUAlloc: n.Status.Allocatable.Cpu().MilliValue(),
			CPUCap:   n.Status.Capacity.Cpu().MilliValue(),
			MemAlloc: n.Status.Allocatable.Memory().Value(),
			MemCap:   n.Status.Capacity.Memory().Value(),
			PodAlloc: n.Status.Allocatable.Pods().Value(),
			PodCap:   n.Status.Capacity.Pods().Value(),
		}
		s.Nodes = append(s.Nodes, fact)
		if fact.Ready {
			s.NodesReady++
			continue
		}
		notReady = append(notReady, n.Name)
	}
	sort.Strings(notReady)
	s.NotReady = notReady
	sort.Slice(s.Nodes, func(i, j int) bool { return s.Nodes[i].Name < s.Nodes[j].Name })
	for _, p := range pods {
		if p == nil {
			continue
		}
		s.PodsTotal++
		if p.Status.Phase == corev1.PodRunning {
			s.PodsRunning++
		}
		reqCPU, limCPU := podCPU(p)
		reqMem, limMem := podMemory(p)
		s.Pods = append(s.Pods, podFact{
			Node:       p.Spec.NodeName,
			Phase:      p.Status.Phase,
			CPURequest: reqCPU,
			CPULimit:   limCPU,
			MemRequest: reqMem,
			MemLimit:   limMem,
		})
	}
	var evs []Event
	for _, e := range events {
		if e == nil || !cluster.EventWarning(e) {
			continue
		}
		s.Warnings++
		evs = append(evs, Event{
			When:    eventWhen(e),
			Object:  eventObject(e),
			Reason:  e.Reason,
			Message: trim(e.Message, 160),
		})
	}
	sort.Slice(evs, func(i, j int) bool {
		if evs[i].When.Equal(evs[j].When) {
			return evs[i].Object < evs[j].Object
		}
		return evs[i].When.After(evs[j].When)
	})
	if len(evs) > 8 {
		evs = evs[:8]
	}
	s.Events = evs
	return s
}

// Point is one bucket of a history. OK false is a gap, not a measured zero.
type Point struct {
	At    time.Time
	Value float64
	OK    bool
}

// Series is usage, requests, and limits against allocatable and capacity.
// UsageOK is false when this window has no metrics sample. Requests and
// limits come from pod specs.
type Series struct {
	Usage, Requests, Limits float64
	Allocatable, Capacity   float64
	UsageOK                 bool
	OverLimit               bool
}

// Presented is what the screen draws for one role, metric, and window.
type Presented struct {
	Role       string
	Metric     string
	Window     Window
	HasMaster  bool
	HasWorker  bool
	CPU        Series
	Memory     Series
	Pods       Series
	Running    int
	Pending    int
	Failed     int
	History    []Point
	CPUHist    []float64
	MemHist    []float64
	Events     []Event
	Start, End time.Time
	MetricsOK  bool
}

// Present filters the snapshot to one node role and one time window.
// It does not invent a usage point for a bucket with no sample.
func Present(s Snapshot, role, metric, window string, samples []Sample, now time.Time) Presented {
	if role != Master {
		role = Worker
	}
	if metric != Memory {
		metric = CPU
	}
	w := WindowBy(window)
	hasMaster, hasWorker := false, false
	names := map[string]bool{}
	var cpu, mem Series
	var pods Series
	for _, n := range s.Nodes {
		if n.Master {
			hasMaster = true
		} else {
			hasWorker = true
		}
		if n.Master != (role == Master) {
			continue
		}
		names[n.Name] = true
		cpu.Allocatable += float64(n.CPUAlloc)
		cpu.Capacity += float64(n.CPUCap)
		mem.Allocatable += float64(n.MemAlloc)
		mem.Capacity += float64(n.MemCap)
		pods.Allocatable += float64(n.PodAlloc)
		pods.Capacity += float64(n.PodCap)
	}
	var running, pending, failed int
	for _, p := range s.Pods {
		if p.Node == "" || !names[p.Node] {
			continue
		}
		cpu.Requests += float64(p.CPURequest)
		cpu.Limits += float64(p.CPULimit)
		mem.Requests += float64(p.MemRequest)
		mem.Limits += float64(p.MemLimit)
		pods.Usage += 1
		pods.UsageOK = true
		switch p.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodPending:
			pending++
		case corev1.PodFailed:
			failed++
		}
	}
	pods.Requests = pods.Usage
	cpu.OverLimit = cpu.Allocatable > 0 && cpu.Limits > cpu.Allocatable
	mem.OverLimit = mem.Allocatable > 0 && mem.Limits > mem.Allocatable
	end := now
	start := now.Add(-w.Dur)
	cpuHist := buckets(samples, names, start, end, true)
	memHist := buckets(samples, names, start, end, false)
	if s.MetricsOK {
		if v, ok := lastOK(cpuHist); ok {
			cpu.Usage, cpu.UsageOK = v, true
		}
		if v, ok := lastOK(memHist); ok {
			mem.Usage, mem.UsageOK = v, true
		}
	}
	hist := cpuHist
	if metric == Memory {
		hist = memHist
	}
	return Presented{
		Role: role, Metric: metric, Window: w,
		HasMaster: hasMaster, HasWorker: hasWorker,
		CPU: cpu, Memory: mem, Pods: pods,
		Running: running, Pending: pending, Failed: failed,
		History: hist, CPUHist: values(cpuHist), MemHist: values(memHist),
		Events: s.Events, Start: start, End: end, MetricsOK: s.MetricsOK,
	}
}

func buckets(samples []Sample, names map[string]bool, start, end time.Time, cpu bool) []Point {
	const n = 16
	out := make([]Point, n)
	span := end.Sub(start)
	if span <= 0 {
		span = time.Second
	}
	step := span / n
	for i := range out {
		out[i].At = start.Add(step*time.Duration(i) + step/2)
		out[i].Value = math.NaN()
	}
	for _, sample := range samples {
		if sample.At.Before(start) || sample.At.After(end) {
			continue
		}
		sum, ok := sumRole(sample, names, cpu)
		if !ok {
			continue
		}
		i := int(sample.At.Sub(start) / step)
		if i < 0 {
			i = 0
		}
		if i >= n {
			i = n - 1
		}
		out[i].Value = sum
		out[i].OK = true
		out[i].At = sample.At
	}
	return out
}

func sumRole(sample Sample, names map[string]bool, cpu bool) (float64, bool) {
	var sum float64
	hit := false
	for _, n := range sample.Nodes {
		if !names[n.Name] {
			continue
		}
		hit = true
		if cpu {
			sum += float64(n.CPUMilli)
		} else {
			sum += float64(n.MemoryBytes)
		}
	}
	return sum, hit
}

func lastOK(points []Point) (float64, bool) {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].OK {
			return points[i].Value, true
		}
	}
	return 0, false
}

func values(points []Point) []float64 {
	out := make([]float64, len(points))
	for i, p := range points {
		if p.OK {
			out[i] = p.Value
			continue
		}
		out[i] = math.NaN()
	}
	return out
}

func masterNode(n *corev1.Node) bool {
	if n == nil {
		return false
	}
	for key := range n.Labels {
		role, ok := nodeRole(key)
		if ok && (role == "master" || role == "control-plane") {
			return true
		}
	}
	return false
}

func nodeRole(key string) (string, bool) {
	const mark = "node-role.kubernetes.io/"
	i := strings.Index(key, mark)
	if i < 0 {
		return "", false
	}
	role := strings.Trim(key[i+len(mark):], "/")
	if role == "" || strings.Contains(role, "/") {
		return "", false
	}
	return role, true
}

func podCPU(p *corev1.Pod) (req, lim int64) {
	if p == nil {
		return 0, 0
	}
	for _, c := range p.Spec.Containers {
		req += c.Resources.Requests.Cpu().MilliValue()
		lim += c.Resources.Limits.Cpu().MilliValue()
	}
	return req, lim
}

func podMemory(p *corev1.Pod) (req, lim int64) {
	if p == nil {
		return 0, 0
	}
	for _, c := range p.Spec.Containers {
		req += c.Resources.Requests.Memory().Value()
		lim += c.Resources.Limits.Memory().Value()
	}
	return req, lim
}

func metricsText(err error, available bool, value string) string {
	if apierrors.IsNotFound(err) || apierrors.IsServiceUnavailable(err) || !available || value == "" {
		return "metrics unavailable"
	}
	return value
}

func formatMilli(milli int64) string {
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

func trim(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func eventWhen(ev *corev1.Event) time.Time {
	switch {
	case !ev.LastTimestamp.IsZero():
		return ev.LastTimestamp.Time
	case !ev.EventTime.IsZero():
		return ev.EventTime.Time
	default:
		return ev.FirstTimestamp.Time
	}
}

func eventObject(ev *corev1.Event) string {
	ref := ev.InvolvedObject
	name := ref.Name
	if ref.Kind != "" && name != "" {
		name = ref.Kind + "/" + name
	}
	if ev.Namespace == "" {
		return name
	}
	if name == "" {
		return ev.Namespace
	}
	return ev.Namespace + "/" + name
}

// Age is a short age, empty when when is zero.
func Age(when, now time.Time) string {
	if when.IsZero() {
		return ""
	}
	d := now.Sub(when)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// Percent is usage against allocatable, or "—" when that ratio is unknown.
func Percent(used, total float64, ok bool) string {
	if !ok || total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", used/total*100)
}

// FormatMilli renders millicores. FormatBytes renders a byte count.
func FormatMilli(v float64) string { return formatMilli(int64(math.Round(v))) }

// FormatBytes renders a byte count.
func FormatBytes(v float64) string { return formatBytes(int64(math.Round(v))) }
