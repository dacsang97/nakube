package namespaces

import (
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
)

const (
	ColStatus = "status"
	ColName   = "name"
	ColLabels = "labels"
	ColAge    = "age"

	All = iota
	Active
	Terminating
)

// Page is the visible namespace list. Total counts namespaces before search and filter.
type Page struct {
	Rows  []Row
	Total int
}

// Row is one namespace. Labels are the full set; the table clips them.
type Row struct {
	Key      string
	Name     string
	Status   string
	Tone     kit.Tone
	Labels   []string
	Age      string
	Detail   kit.Detail
	Identity cluster.Identity

	created time.Time
}

// Filters is the phase switch.
func Filters() []string { return []string{"All", "Active", "Terminating"} }

// Build lists namespaces. An empty phase is Active, which is the phase a
// namespace has until it is deleted. column empty sorts by name.
func Build(items []*corev1.Namespace, query string, filter int, column string, desc bool, now time.Time) Page {
	rows := make([]Row, 0, len(items))
	total := 0
	for _, item := range items {
		if item == nil {
			continue
		}
		total++
		row := rowFrom(item, now)
		if !kit.Match(query, row.Name, row.Status, strings.Join(row.Labels, " ")) || !phaseFilter(row.Status, filter) {
			continue
		}
		rows = append(rows, row)
	}
	sortRows(rows, column, desc)
	return Page{Rows: rows, Total: total}
}

func rowFrom(item *corev1.Namespace, now time.Time) Row {
	status, tone := phase(item.Status.Phase)
	labels := labelLines(item.Labels)
	row := Row{
		Key:     item.Name,
		Name:    item.Name,
		Status:  status,
		Tone:    tone,
		Labels:  labels,
		Age:     kit.Age(item.CreationTimestamp.Time, now),
		created: item.CreationTimestamp.Time,
		Identity: cluster.Identity{
			GVR:  schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
			Kind: "Namespace",
			Name: item.Name,
		},
	}
	row.Detail = kit.Detail{
		Title: item.Name,
		Facts: []kit.Fact{
			{Label: "Status", Value: status, Tone: tone},
			{Label: "Age", Value: row.Age},
		},
		Groups: []kit.Group{
			{Title: "Labels", Chips: labelChips(labels)},
			{Title: "Finalizers", Lines: finalizers(item.Spec.Finalizers)},
		},
	}
	return row
}

func phase(value corev1.NamespacePhase) (string, kit.Tone) {
	switch value {
	case "", corev1.NamespaceActive:
		return "Active", kit.ToneOK
	case corev1.NamespaceTerminating:
		return "Terminating", kit.ToneBad
	default:
		return string(value), kit.ToneMuted
	}
}

func phaseFilter(status string, filter int) bool {
	switch filter {
	case Active:
		return status == "Active"
	case Terminating:
		return status == "Terminating"
	default:
		return true
	}
}

func labelLines(labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, len(keys))
	for i, key := range keys {
		out[i] = key + "=" + labels[key]
	}
	return out
}

func labelChips(labels []string) []kit.Chip {
	out := make([]kit.Chip, len(labels))
	for i, label := range labels {
		out[i] = kit.Chip{Text: label}
	}
	return out
}

func finalizers(values []corev1.FinalizerName) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, string(value))
		}
	}
	return out
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
	case ColStatus:
		return kit.CompareText(a.Status, b.Status, desc)
	case ColLabels:
		return kit.CompareText(strings.Join(a.Labels, " "), strings.Join(b.Labels, " "), desc)
	case ColAge:
		return kit.CompareTime(a.created, b.created, desc)
	default:
		return kit.CompareText(a.Name, b.Name, desc)
	}
}
