package nodes

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
)

const (
	ColStatus     = "status"
	ColName       = "name"
	ColRoles      = "roles"
	ColCPU        = "cpu"
	ColMemory     = "memory"
	ColVersion    = "version"
	ColConditions = "conditions"
	ColTaints     = "taints"
	ColAge        = "age"
)

const (
	All = iota
	Ready
	NotReady
)

// Sample is one node's metrics reading. A node absent from the map was not
// in the sample; it is not a zero.
type Sample struct {
	CPUMilli    int64
	MemoryBytes int64
}

// Page is the visible node list. Total counts the nodes before search and filter.
type Page struct {
	Rows  []Row
	Total int
}

// Row is one node. Detail is what the table cannot show.
type Row struct {
	Key        string
	Name       string
	Status     string
	StatusTone kit.Tone
	Roles      string
	Version    string
	Age        string
	CPU        float64
	CPUMax     float64
	CPUKnown   bool
	CPUTip     string
	Mem        float64
	MemMax     float64
	MemKnown   bool
	MemTip     string
	Conditions []kit.Chip
	Taints     string
	Detail     kit.Detail
	Identity   cluster.Identity

	created  time.Time
	condText string
	taintN   int
	search   string
}

// Filters is the status switch.
func Filters() []string { return []string{"All", "Ready", "Not ready"} }

// Build lists nodes. column and desc are the table sort; an empty column
// sorts by name. Metrics that were not sampled stay unknown.
func Build(items []*corev1.Node, usage map[string]Sample, query string, filter int, column string, desc bool, now time.Time) Page {
	rows := make([]Row, 0, len(items))
	total := 0
	for _, node := range items {
		if node == nil {
			continue
		}
		total++
		row := rowFrom(node, usage[node.Name], usageHas(usage, node.Name), now)
		if !kit.Match(query, row.search) || !statusFilter(row.Status, filter) {
			continue
		}
		rows = append(rows, row)
	}
	sortRows(rows, column, desc)
	return Page{Rows: rows, Total: total}
}

func usageHas(usage map[string]Sample, name string) bool {
	if usage == nil {
		return false
	}
	_, ok := usage[name]
	return ok
}

func rowFrom(node *corev1.Node, sample Sample, known bool, now time.Time) Row {
	status, tone := nodeStatus(node)
	roleList := roleNames(node.Labels)
	conds := activeConditions(node)
	taints := taintLines(node.Spec.Taints)
	version := node.Status.NodeInfo.KubeletVersion
	cpuMax, cpuOK := quantityMilli(node.Status.Allocatable, corev1.ResourceCPU)
	memMax, memOK := quantityBytes(node.Status.Allocatable, corev1.ResourceMemory)
	row := Row{
		Key:        node.Name,
		Name:       node.Name,
		Status:     status,
		StatusTone: tone,
		Roles:      strings.Join(roleList, ", "),
		Version:    version,
		Age:        kit.Age(node.CreationTimestamp.Time, now),
		Taints:     strconv.Itoa(len(taints)),
		Conditions: condChips(conds),
		created:    node.CreationTimestamp.Time,
		condText:   condText(conds),
		taintN:     len(taints),
		Identity: cluster.Identity{
			GVR:  schema.GroupVersionResource{Version: "v1", Resource: "nodes"},
			Kind: "Node",
			Name: node.Name,
		},
	}
	if known && cpuOK {
		row.CPUKnown = true
		row.CPU = float64(sample.CPUMilli)
		row.CPUMax = float64(cpuMax)
		row.CPUTip = usageTip("CPU", milliText(sample.CPUMilli), milliText(cpuMax), row.CPU, row.CPUMax)
	}
	if known && memOK {
		row.MemKnown = true
		row.Mem = float64(sample.MemoryBytes)
		row.MemMax = float64(memMax)
		row.MemTip = usageTip("Memory", byteText(sample.MemoryBytes), byteText(memMax), row.Mem, row.MemMax)
	}
	row.search = strings.Join([]string{
		row.Name, row.Status, row.Roles, row.Version, row.condText, strings.Join(taints, " "),
		strings.Join(addresses(node), " "),
	}, " ")
	row.Detail = detail(node, row, conds, taints)
	return row
}

func detail(node *corev1.Node, row Row, conds []cond, taints []string) kit.Detail {
	facts := []kit.Fact{
		{Label: "Status", Value: row.Status, Tone: row.StatusTone},
		{Label: "Roles", Value: row.Roles},
		{Label: "Age", Value: row.Age},
	}
	if cidr := node.Spec.PodCIDR; cidr != "" {
		facts = append(facts, kit.Fact{Label: "Pod CIDR", Value: cidr})
	}
	groups := []kit.Group{
		{Title: "Capacity", Facts: resourceFacts(node.Status.Capacity)},
		{Title: "Allocatable", Facts: resourceFacts(node.Status.Allocatable)},
		{Title: "Conditions", Chips: row.Conditions, Lines: condMessages(conds)},
		{Title: "Taints", Lines: taints},
		{Title: "System", Facts: systemFacts(node)},
		{Title: "Addresses", Lines: addresses(node)},
	}
	return kit.Detail{Title: row.Name, Facts: facts, Groups: groups}
}

func statusFilter(status string, filter int) bool {
	switch filter {
	case Ready:
		return status == "Ready"
	case NotReady:
		return status != "Ready"
	default:
		return true
	}
}

func nodeStatus(node *corev1.Node) (string, kit.Tone) {
	for _, cond := range node.Status.Conditions {
		if cond.Type != corev1.NodeReady {
			continue
		}
		switch cond.Status {
		case corev1.ConditionTrue:
			return "Ready", kit.ToneOK
		case corev1.ConditionFalse:
			return "NotReady", kit.ToneBad
		default:
			return "Unknown", kit.ToneMuted
		}
	}
	return "Unknown", kit.ToneMuted
}

func roleNames(labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out []string
	seen := map[string]bool{}
	add := func(role string) {
		if role == "" || seen[role] {
			return
		}
		seen[role] = true
		out = append(out, role)
	}
	const prefix = "node-role.kubernetes.io/"
	for _, key := range keys {
		if strings.HasPrefix(key, prefix) {
			add(strings.TrimPrefix(key, prefix))
		}
	}
	add(labels["kubernetes.io/role"])
	add(labels["node.kubernetes.io/role"])
	return out
}

type cond struct {
	Type, Message string
	Tone          kit.Tone
}

func activeConditions(node *corev1.Node) []cond {
	var out []cond
	if node.Spec.Unschedulable {
		out = append(out, cond{Type: "SchedulingDisabled", Tone: kit.ToneWarn})
	}
	items := append([]corev1.NodeCondition(nil), node.Status.Conditions...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Type == corev1.NodeReady {
			return true
		}
		if items[j].Type == corev1.NodeReady {
			return false
		}
		return items[i].Type < items[j].Type
	})
	for _, item := range items {
		if item.Status != corev1.ConditionTrue {
			continue
		}
		out = append(out, cond{Type: string(item.Type), Message: item.Message, Tone: conditionTone(string(item.Type))})
	}
	return out
}

func conditionTone(kind string) kit.Tone {
	switch kind {
	case "Ready":
		return kit.ToneOK
	case "MemoryPressure", "DiskPressure", "PIDPressure", "SchedulingDisabled":
		return kit.ToneWarn
	case "OutOfDisk", "NetworkUnavailable":
		return kit.ToneBad
	default:
		return kit.ToneMuted
	}
}

func condChips(conds []cond) []kit.Chip {
	out := make([]kit.Chip, len(conds))
	for i, item := range conds {
		out[i] = kit.Chip{Text: item.Type, Tone: item.Tone}
	}
	return out
}

func condText(conds []cond) string {
	names := make([]string, len(conds))
	for i, item := range conds {
		names[i] = item.Type
	}
	return strings.Join(names, " ")
}

func condMessages(conds []cond) []string {
	var out []string
	for _, item := range conds {
		if item.Message == "" {
			continue
		}
		out = append(out, item.Type+": "+item.Message)
	}
	return out
}

func taintLines(taints []corev1.Taint) []string {
	out := make([]string, 0, len(taints))
	for _, taint := range taints {
		if taint.Key == "" {
			continue
		}
		line := taint.Key
		if taint.Value != "" {
			line += "=" + taint.Value
		}
		if taint.Effect != "" {
			line += ":" + string(taint.Effect)
		}
		out = append(out, line)
	}
	return out
}

func addresses(node *corev1.Node) []string {
	out := make([]string, 0, len(node.Status.Addresses))
	for _, addr := range node.Status.Addresses {
		if addr.Address == "" {
			continue
		}
		out = append(out, string(addr.Type)+": "+addr.Address)
	}
	return out
}

func systemFacts(node *corev1.Node) []kit.Fact {
	info := node.Status.NodeInfo
	return []kit.Fact{
		{Label: "OS", Value: nodeOS(node)},
		{Label: "OS image", Value: info.OSImage},
		{Label: "Kernel", Value: info.KernelVersion},
		{Label: "Runtime", Value: info.ContainerRuntimeVersion},
		{Label: "Kubelet", Value: info.KubeletVersion},
	}
}

func nodeOS(node *corev1.Node) string {
	for _, key := range []string{"kubernetes.io/os", "beta.kubernetes.io/os"} {
		if value := node.Labels[key]; value != "" {
			return value
		}
	}
	return node.Status.NodeInfo.OperatingSystem
}

func resourceFacts(list corev1.ResourceList) []kit.Fact {
	return []kit.Fact{
		{Label: "CPU", Value: cpuResource(list)},
		{Label: "Memory", Value: bytesResource(list, corev1.ResourceMemory)},
		{Label: "Ephemeral", Value: bytesResource(list, corev1.ResourceEphemeralStorage)},
		{Label: "Pods", Value: countResource(list, corev1.ResourcePods)},
	}
}

func cpuResource(list corev1.ResourceList) string {
	q, ok := list[corev1.ResourceCPU]
	if !ok {
		return ""
	}
	return milliText(q.MilliValue())
}

func bytesResource(list corev1.ResourceList, name corev1.ResourceName) string {
	q, ok := list[name]
	if !ok {
		return ""
	}
	return byteText(q.Value())
}

func countResource(list corev1.ResourceList, name corev1.ResourceName) string {
	q, ok := list[name]
	if !ok {
		return ""
	}
	return q.String()
}

func quantityMilli(list corev1.ResourceList, name corev1.ResourceName) (int64, bool) {
	q, ok := list[name]
	if !ok || q.MilliValue() <= 0 {
		return 0, false
	}
	return q.MilliValue(), true
}

func quantityBytes(list corev1.ResourceList, name corev1.ResourceName) (int64, bool) {
	q, ok := list[name]
	if !ok || q.Value() <= 0 {
		return 0, false
	}
	return q.Value(), true
}

func usageTip(kind, used, total string, value, hi float64) string {
	if hi <= 0 {
		return kind
	}
	return fmt.Sprintf("%s %s / %s, %.0f%%", kind, used, total, value/hi*100)
}

func milliText(milli int64) string {
	if milli%1000 == 0 {
		return strconv.FormatInt(milli/1000, 10)
	}
	return strconv.FormatInt(milli, 10) + "m"
}

func byteText(n int64) string {
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

func sortRows(rows []Row, column string, desc bool) {
	if column == "" {
		column = ColName
	}
	sort.SliceStable(rows, func(i, j int) bool {
		c := compare(rows[i], rows[j], column, desc)
		if c == 0 {
			return rows[i].Name < rows[j].Name
		}
		return c < 0
	})
}

func compare(a, b Row, column string, desc bool) int {
	switch column {
	case ColCPU:
		return kit.CompareKnown(a.CPUKnown, b.CPUKnown, a.CPU, b.CPU, desc)
	case ColMemory:
		return kit.CompareKnown(a.MemKnown, b.MemKnown, a.Mem, b.Mem, desc)
	case ColAge:
		return kit.CompareTime(a.created, b.created, desc)
	case ColTaints:
		return kit.CompareFloat(float64(a.taintN), float64(b.taintN), desc)
	case ColStatus:
		return kit.CompareText(a.Status, b.Status, desc)
	case ColRoles:
		return kit.CompareText(a.Roles, b.Roles, desc)
	case ColVersion:
		return kit.CompareText(a.Version, b.Version, desc)
	case ColConditions:
		return kit.CompareText(a.condText, b.condText, desc)
	default:
		return kit.CompareText(a.Name, b.Name, desc)
	}
}
