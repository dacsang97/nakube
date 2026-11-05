package desktop

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
)

func TestSidebarOmitsDiagramAndFlux(t *testing.T) {
	ids := SidebarIDs()
	if len(ids) == 0 {
		t.Fatal("empty sidebar")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "resource-diagram" || id == "flux" || strings.Contains(id, "flux") {
			t.Fatalf("forbidden sidebar id %s", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
	}
	for _, want := range []string{"overview", "pods", "helm", "argo-applications", "istio-virtualservices"} {
		if !seen[want] {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestMetricsCardNotFound(t *testing.T) {
	err := apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "nodes"}, "nodes")
	if got := metricsCardText(err, false, ""); got != "metrics unavailable" {
		t.Fatalf("card text = %q", got)
	}
	if got := metricsCardText(nil, true, "12m"); got != "12m" {
		t.Fatalf("available card = %q", got)
	}
}

func TestNoteUsageNotFoundDoesNotInventNumber(t *testing.T) {
	a := New(cluster.NewSession())
	err := apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "nodes"}, "nodes")
	a.noteUsage(cluster.Usage{}, err)
	o := overviewFrom(nil, nil, nil, a.usage, a.usageErr)
	if o.CPU != "metrics unavailable" || o.Memory != "metrics unavailable" {
		t.Fatalf("cpu=%q memory=%q", o.CPU, o.Memory)
	}
	if strings.Contains(o.CPU, "0m") || strings.Contains(o.Memory, "0") {
		t.Fatalf("invented a number: %+v", o)
	}
}

func TestConfirmIgnoresLaterSelection(t *testing.T) {
	a := New(cluster.NewSession())
	a.OpenConfirm("Pod", "ns", "web-0", "delete")
	a.mu.Lock()
	a.selected = "pods"
	a.mu.Unlock()
	got, ok := a.ConfirmIdentity()
	if !ok || got.Name != "web-0" || got.Namespace != "ns" || got.Kind != "Pod" {
		t.Fatalf("confirm retargeted: %+v ok=%v", got, ok)
	}
}

func TestWatchFoldKeepsCopy(t *testing.T) {
	a := New(cluster.NewSession())
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	a.onWatch(cluster.WatchEvent{Type: "add", Kind: "Pod", Object: pod})
	pod.Status.Phase = corev1.PodFailed
	a.mu.Lock()
	got := a.pods["ns/web"]
	a.mu.Unlock()
	if got == nil || got.Status.Phase != corev1.PodRunning {
		t.Fatal("fold stored the live object")
	}
}

func TestDarkUsesSameSidebar(t *testing.T) {
	light := SidebarIDs()
	if NextAppearance(AppearanceLight) != AppearanceDark {
		t.Fatal("dark toggle")
	}
	if NextAppearance(AppearanceDark) != AppearanceLight {
		t.Fatal("light toggle")
	}
	dark := SidebarIDs()
	if strings.Join(light, ",") != strings.Join(dark, ",") {
		t.Fatal("dark changed the sidebar")
	}
}

func TestEmptyContextView(t *testing.T) {
	a := New(cluster.NewSession())
	a.errText = "no kube context is configured"
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("no kube context is configured") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("Overview") {
		t.Fatalf("missing overview, texts %q", tt.Texts())
	}
	if !tt.HasText("metrics unavailable") {
		t.Fatalf("missing metrics card, texts %q", tt.Texts())
	}
}

func TestPublicErrorHidesToken(t *testing.T) {
	got := publicError(errString("bearer token abc"))
	if strings.Contains(got, "abc") || strings.Contains(strings.ToLower(got), "token") {
		t.Fatalf("leaked token: %q", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
