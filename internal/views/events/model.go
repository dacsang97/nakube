package events

import (
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
	ColType      = "type"
	ColReason    = "reason"
	ColObject    = "object"
	ColMessage   = "message"
	ColNamespace = "namespace"
	ColSource    = "source"
	ColCount     = "count"
	ColAge       = "age"
)

// Page is the visible event list. Total counts events before search.
type Page struct {
	Rows  []Row
	Total int
}

// Row is one event. Message is the full text; the table clips it.
type Row struct {
	Key       string
	Type      string
	TypeTone  kit.Tone
	Reason    string
	Object    string
	Message   string
	Namespace string
	Source    string
	Count     string
	Age       string
	Warning   bool
	Detail    kit.Detail
	Identity  cluster.Identity

	seen  time.Time
	count int
}

// Build lists events. column empty sorts by last seen, newest first, because
// an event name is not a useful order. A missing last time uses the series
// time, then the event time, then the first time.
func Build(items []*corev1.Event, query, column string, desc bool, now time.Time, namespace string) Page {
	if column == "" {
		column = ColAge
		desc = true
	}
	rows := make([]Row, 0, len(items))
	total := 0
	for _, item := range items {
		if item == nil || namespace != "" && item.Namespace != namespace {
			continue
		}
		total++
		row := rowFrom(item, now)
		if !kit.Match(query, row.Type, row.Reason, row.Object, row.Message, row.Namespace, row.Source) {
			continue
		}
		rows = append(rows, row)
	}
	sortRows(rows, column, desc)
	return Page{Rows: rows, Total: total}
}

func rowFrom(item *corev1.Event, now time.Time) Row {
	seen := lastSeen(item)
	first := firstSeen(item)
	warning := item.Type == corev1.EventTypeWarning
	tone := kit.ToneMuted
	if warning {
		tone = kit.ToneBad
	}
	object := objectText(item.InvolvedObject)
	count, countText := eventCount(item)
	row := Row{
		Key:       item.Namespace + "/" + item.Name,
		Type:      item.Type,
		TypeTone:  tone,
		Reason:    item.Reason,
		Object:    object,
		Message:   strings.Join(strings.Fields(item.Message), " "),
		Namespace: item.Namespace,
		Source:    sourceText(item),
		Count:     countText,
		Age:       kit.Age(seen, now),
		Warning:   warning,
		seen:      seen,
		count:     count,
		Identity: cluster.Identity{
			GVR:       schema.GroupVersionResource{Version: "v1", Resource: "events"},
			Kind:      "Event",
			Namespace: item.Namespace,
			Name:      item.Name,
		},
	}
	facts := []kit.Fact{
		{Label: "Type", Value: row.Type, Tone: tone},
		{Label: "Reason", Value: row.Reason},
		{Label: "Object", Value: object},
		{Label: "Namespace", Value: row.Namespace},
		{Label: "Source", Value: row.Source},
		{Label: "Count", Value: countText},
		{Label: "First seen", Value: kit.When(first, now)},
		{Label: "Last seen", Value: kit.When(seen, now)},
	}
	if path := item.InvolvedObject.FieldPath; path != "" {
		facts = append(facts, kit.Fact{Label: "Field", Value: path})
	}
	title := object
	if title == "" {
		title = item.Reason
	}
	row.Detail = kit.Detail{
		Title: title,
		Facts: facts,
		Groups: []kit.Group{{
			Title: "Message",
			Lines: messageLines(item.Message),
		}},
	}
	return row
}

func lastSeen(item *corev1.Event) time.Time {
	switch {
	case !item.LastTimestamp.IsZero():
		return item.LastTimestamp.Time
	case item.Series != nil && !item.Series.LastObservedTime.IsZero():
		return item.Series.LastObservedTime.Time
	case !item.EventTime.IsZero():
		return item.EventTime.Time
	default:
		return item.FirstTimestamp.Time
	}
}

func firstSeen(item *corev1.Event) time.Time {
	if !item.FirstTimestamp.IsZero() {
		return item.FirstTimestamp.Time
	}
	return item.EventTime.Time
}

func objectText(ref corev1.ObjectReference) string {
	switch {
	case ref.Kind != "" && ref.Name != "":
		return ref.Kind + "/" + ref.Name
	case ref.Name != "":
		return ref.Name
	default:
		return ref.Kind
	}
}

func sourceText(item *corev1.Event) string {
	component := item.Source.Component
	host := item.Source.Host
	if component == "" {
		component = item.ReportingController
	}
	if component == "" {
		return ""
	}
	if host == "" {
		return component
	}
	return component + " " + host
}

func eventCount(item *corev1.Event) (int, string) {
	n := int(item.Count)
	if item.Series != nil && int(item.Series.Count) > n {
		n = int(item.Series.Count)
	}
	if n <= 0 {
		return -1, ""
	}
	return n, strconv.Itoa(n)
}

func messageLines(message string) []string {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	return []string{message}
}

func sortRows(rows []Row, column string, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		c := compare(rows[i], rows[j], column, desc)
		if c == 0 {
			return rows[i].Key < rows[j].Key
		}
		return c < 0
	})
}

func compare(a, b Row, column string, desc bool) int {
	switch column {
	case ColAge:
		return kit.CompareTime(a.seen, b.seen, desc)
	case ColCount:
		return kit.CompareKnown(a.count >= 0, b.count >= 0, float64(a.count), float64(b.count), desc)
	case ColType:
		return kit.CompareText(a.Type, b.Type, desc)
	case ColReason:
		return kit.CompareText(a.Reason, b.Reason, desc)
	case ColObject:
		return kit.CompareText(a.Object, b.Object, desc)
	case ColMessage:
		return kit.CompareText(a.Message, b.Message, desc)
	case ColNamespace:
		return kit.CompareText(a.Namespace, b.Namespace, desc)
	case ColSource:
		return kit.CompareText(a.Source, b.Source, desc)
	default:
		return kit.CompareTime(a.seen, b.seen, true)
	}
}
