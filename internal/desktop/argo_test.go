package desktop

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
)

func TestMissingCRDNotice(t *testing.T) {
	if emptyText("port-forwards") == "None" {
		t.Fatal("port-forward empty text is the generic one")
	}
	if emptyText("services") != "None" {
		t.Fatalf("services empty = %q", emptyText("services"))
	}
}

func TestArgoDashboardFrame(t *testing.T) {
	a := New(cluster.NewSession())
	a.router = ui.NewRouter("/argo")
	a.selected = "argo"
	a.listID = "argo"
	a.current = "dev"
	a.argoApps = []cluster.ArgoApp{{
		Name:      "shop",
		Namespace: "argo",
		Sync:      "OutOfSync",
		Health:    "Degraded",
	}}
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	a.events["argo/shop"] = &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "shop", Namespace: "argo"},
		Reason:     "ComparisonError",
		Message:    "denied",
		Type:       corev1.EventTypeWarning,
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Application",
			Namespace: "argo",
			Name:      "shop",
		},
		LastTimestamp: metav1.NewTime(now.Add(-time.Minute)),
	}
	tt := ui.NewTester(a.view, 1280, 900)
	for _, want := range []string{"Argo", "Out of sync", "Degraded", "shop", "Needs attention", "ComparisonError"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText("Install") {
		t.Fatal("dashboard offers to install Argo")
	}
}
