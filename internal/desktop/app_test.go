package desktop

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/overview"
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
	missing := overview.From(nil, nil, nil, cluster.Usage{}, err, time.Now())
	if missing.CPU != "metrics unavailable" || missing.Memory != "metrics unavailable" {
		t.Fatalf("card text %+v", missing)
	}
	got := overview.From(nil, nil, nil, cluster.Usage{Available: true, CPUMilli: 12}, nil, time.Now())
	if got.CPU != "12m" {
		t.Fatalf("available card = %q", got.CPU)
	}
}

func TestNoteUsageNotFoundDoesNotInventNumber(t *testing.T) {
	a := New(cluster.NewSession())
	err := apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "nodes"}, "nodes")
	a.noteUsage(cluster.Usage{}, err)
	o := overview.From(nil, nil, nil, a.usage, a.usageErr, time.Now())
	if o.CPU != "metrics unavailable" || o.Memory != "metrics unavailable" {
		t.Fatalf("cpu=%q memory=%q", o.CPU, o.Memory)
	}
	if strings.Contains(o.CPU, "0m") || strings.Contains(o.Memory, "0") {
		t.Fatalf("invented a number: %+v", o)
	}
	if len(a.samples) != 0 {
		t.Fatal("a missing API still recorded a sample")
	}
}

func TestStartWatchReleasesTheLock(t *testing.T) {
	a := New(cluster.NewSession())
	done := make(chan struct{})
	go func() {
		a.startWatch(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startWatch blocked while holding the app lock")
	}
	if !a.mu.TryLock() {
		t.Fatal("app lock still held after startWatch")
	}
	released := a.watchCancel != nil || a.errText != ""
	a.mu.Unlock()
	if !released {
		t.Fatal("startWatch neither opened a watch nor recorded an error")
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
	ids := SidebarIDs()
	if len(ids) == 0 {
		t.Fatal("empty sidebar")
	}
}

func TestEmptyContextView(t *testing.T) {
	a := New(cluster.NewSession())
	a.errText = "no kube context is configured"
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("no kube context is configured") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if !tt.HasText("NaKube") || !tt.HasText("Overview") || !tt.HasText("Workloads") || !tt.HasText("Pods") {
		t.Fatalf("missing menu, texts %q", tt.Texts())
	}
	if !tt.HasText("metrics unavailable") {
		t.Fatalf("missing metrics card, texts %q", tt.Texts())
	}
}

func TestMenuSearchFilters(t *testing.T) {
	a := New(cluster.NewSession())
	a.menuQuery = "virtual"
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("VirtualServices") {
		t.Fatalf("missing match, texts %q", tt.Texts())
	}
	if tt.HasText("DaemonSets") || tt.HasText("Workloads") {
		t.Fatalf("unrelated item stayed, texts %q", tt.Texts())
	}
}

func TestCatalogPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := New(cluster.NewSession())
	a.router.Push("/catalog")
	tt := ui.NewTester(a.view, 1100, 720)
	for _, want := range []string{"Catalog", "Done", "Add file", "Add pasted kubeconfig", "No contexts", "~/.kube"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q, texts %q", want, tt.Texts())
		}
	}
}

func TestSettingsPage(t *testing.T) {
	a := New(cluster.NewSession())
	a.router.Push("/settings")
	tt := ui.NewTester(a.view, 1100, 720)
	for _, want := range []string{"Settings", "Appearance", "System", "Light", "Dark"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q, texts %q", want, tt.Texts())
		}
	}
	if _, ok := parseAppearance("nope"); ok {
		t.Fatal("accepted an unknown appearance")
	}
	path := t.TempDir() + "/appearance"
	if err := writeAppearance(path, mygo.ThemeDark); err != nil {
		t.Fatal(err)
	}
	if got := readAppearance(path); got != mygo.ThemeDark {
		t.Fatalf("read %s", got)
	}
}

func TestSidebarIcons(t *testing.T) {
	seen := map[*ui.SVG]bool{}
	for _, id := range SidebarIDs() {
		icon := IconFor(id)
		if icon == nil {
			t.Fatalf("%s has no icon", id)
		}
		w, h := icon.Size()
		if w != 24 || h != 24 {
			t.Fatalf("%s icon size %v,%v", id, w, h)
		}
		seen[icon] = true
	}
	if len(seen) < 20 {
		t.Fatalf("only %d distinct icons", len(seen))
	}
	if IconNamed("plus") == nil {
		t.Fatal("the rail has no plus icon")
	}
}

func TestSidebarScrolls(t *testing.T) {
	a := New(cluster.NewSession())
	a.router = ui.NewRouter("/istio-virtualservices")
	tt := ui.NewTester(a.view, 400, 240)
	before, ok := tt.Find("RequestAuthentications")
	if !ok {
		t.Fatalf("missing item, texts %q", tt.Texts())
	}
	if before.Y+before.H <= 240 {
		t.Fatalf("item already in view at %v", before)
	}
	tt.Scroll(140, 160, 0, 800)
	tt.Frame()
	after, _ := tt.Find("RequestAuthentications")
	if after.Y >= before.Y-1 {
		t.Fatalf("sidebar did not scroll: before %v after %v", before, after)
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
