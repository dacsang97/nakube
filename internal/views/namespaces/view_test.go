package namespaces

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDrawShowsPhaseAndLabel(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	item := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "team",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
			Labels:            map[string]string{"app": "shop"},
		},
		Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	page := Build([]*corev1.Namespace{item}, "", All, ColName, false, now)
	query := ""
	filter := All
	selected := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), page, &query, &filter, &ui.ListState{Selected: &selected}, false, nil)
	}, 1100, 700)
	for _, want := range []string{"Namespaces", "team", "Active", "app=shop"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	for _, text := range tt.Texts() {
		if text == "Namespace" {
			t.Fatalf("cluster scope drew an empty namespace column %v", tt.Texts())
		}
	}
}
