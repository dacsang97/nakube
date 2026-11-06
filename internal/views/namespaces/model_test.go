package namespaces

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kit "github.com/dacsang97/nakube/internal/ui"
)

func TestBuildKeepsPhaseAndLabels(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	team := ns("team", corev1.NamespaceActive, now.Add(-time.Hour))
	team.Labels = map[string]string{"app": "shop"}
	team.Spec.Finalizers = []corev1.FinalizerName{"kubernetes"}
	page := Build([]*corev1.Namespace{team, ns("old", corev1.NamespaceTerminating, now), nil}, "", All, ColName, false, now)
	if page.Total != 2 || len(page.Rows) != 2 || page.Rows[0].Name != "old" {
		t.Fatalf("page %+v", page.Rows)
	}
	row := page.Rows[1]
	if row.Status != "Active" || row.Tone != kit.ToneOK || len(row.Labels) != 1 || row.Labels[0] != "app=shop" {
		t.Fatalf("row %+v", row)
	}
	if row.Detail.Groups[1].Lines[0] != "kubernetes" {
		t.Fatalf("detail %+v", row.Detail)
	}
	if row.Identity.Kind != "Namespace" || row.Identity.Namespace != "" {
		t.Fatalf("identity %+v", row.Identity)
	}
}

func TestEmptyPhaseIsActive(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	page := Build([]*corev1.Namespace{ns("plain", "", now)}, "", Active, ColName, false, now)
	if len(page.Rows) != 1 || page.Rows[0].Status != "Active" {
		t.Fatalf("%+v", page.Rows)
	}
}

func TestTerminatingFilterAndSearch(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	team := ns("team", corev1.NamespaceActive, now)
	team.Labels = map[string]string{"app": "shop"}
	items := []*corev1.Namespace{team, ns("old", corev1.NamespaceTerminating, now)}
	filtered := Build(items, "", Terminating, ColName, false, now)
	if len(filtered.Rows) != 1 || filtered.Rows[0].Name != "old" || filtered.Total != 2 {
		t.Fatalf("filter %+v", filtered.Rows)
	}
	found := Build(items, "shop", All, ColName, false, now)
	if len(found.Rows) != 1 || found.Rows[0].Name != "team" {
		t.Fatalf("search %+v", found.Rows)
	}
}

func ns(name string, phase corev1.NamespacePhase, created time.Time) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created)},
		Status:     corev1.NamespaceStatus{Phase: phase},
	}
}
