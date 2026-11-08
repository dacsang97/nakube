package events

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildSortsNewestWarningFirst(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	old := event("ns", "old", corev1.EventTypeNormal, "Pulled", "image pulled", now.Add(-time.Hour))
	old.InvolvedObject = corev1.ObjectReference{Kind: "Pod", Name: "web"}
	old.Source = corev1.EventSource{Component: "kubelet", Host: "cp"}
	old.Count = 2
	recent := event("ns", "recent", corev1.EventTypeWarning, "Failed", "secret-token was not found\non the pod", now.Add(-2*time.Minute))
	recent.InvolvedObject = corev1.ObjectReference{Kind: "Pod", Name: "api", FieldPath: "spec.containers{api}"}
	recent.Series = &corev1.EventSeries{Count: 4, LastObservedTime: metav1.NewMicroTime(now.Add(-time.Minute))}
	recent.LastTimestamp = metav1.NewTime(now.Add(-2 * time.Minute))
	page := Build([]*corev1.Event{old, recent, nil}, "", "", false, now, "")
	if page.Total != 2 || len(page.Rows) != 2 || page.Rows[0].Reason != "Failed" {
		t.Fatalf("order %+v", page.Rows)
	}
	row := page.Rows[0]
	if !row.Warning || row.Object != "Pod/api" || row.Count != "4" || row.Age != "2m" {
		t.Fatalf("warning %+v", row)
	}
	if row.Message != "secret-token was not found on the pod" || row.Detail.Facts[7].Value == "" {
		t.Fatalf("message %q detail %+v", row.Message, row.Detail.Facts)
	}
	if page.Rows[1].Source != "kubelet cp" || page.Rows[1].Count != "2" {
		t.Fatalf("source %+v", page.Rows[1])
	}
}

func TestSearchUsesObjectNotEventName(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	item := event("shop", "shop.18a", corev1.EventTypeWarning, "BackOff", "back-off restarting", now)
	item.InvolvedObject = corev1.ObjectReference{Kind: "Pod", Name: "api"}
	page := Build([]*corev1.Event{item}, "api", ColAge, true, now, "")
	if len(page.Rows) != 1 {
		t.Fatal("object search missed")
	}
	miss := Build([]*corev1.Event{item}, "shop.18a", ColAge, true, now, "")
	if len(miss.Rows) != 0 || miss.Total != 1 {
		t.Fatalf("event name was searchable %+v", miss)
	}
}

func event(namespace, name, typ, reason, message string, when time.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:           typ,
		Reason:         reason,
		Message:        message,
		LastTimestamp:  metav1.NewTime(when),
		FirstTimestamp: metav1.NewTime(when.Add(-time.Minute)),
	}
}

func TestBuildNamespaceFilter(t *testing.T) {
	now := time.Now()
	items := []*corev1.Event{
		{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns-b"}},
	}
	page := Build(items, "", ColAge, false, now, "ns-a")
	if len(page.Rows) != 1 || page.Total != 1 || page.Rows[0].Key != "ns-a/a" {
		t.Fatalf("namespace page = %+v", page)
	}
	all := Build(items, "", ColAge, false, now, "")
	if len(all.Rows) != 2 || all.Total != 2 {
		t.Fatalf("all page = %+v", all)
	}
}
