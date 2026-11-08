package events

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDrawShowsWarningObjectAndMessage(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	item := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "shop.18a", Namespace: "shop"},
		Type:           corev1.EventTypeWarning,
		Reason:         "Failed",
		Message:        "secret-token was not found",
		LastTimestamp:  metav1.NewTime(now.Add(-2 * time.Minute)),
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "api"},
		Count:          3,
	}
	page := Build([]*corev1.Event{item}, "", ColAge, true, now, "")
	query := ""
	selected := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), page, &query, &ui.ListState{Selected: &selected}, false, nil)
	}, 1100, 700)
	for _, want := range []string{"Events", "Warning", "Failed", "Pod/api", "secret-token was not found", "3"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText("shop.18a") {
		t.Fatalf("event name replaced the object %v", tt.Texts())
	}
}
