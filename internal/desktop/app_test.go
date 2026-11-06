package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/argo"
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

func TestWorkloadsPageUsesWatchedResources(t *testing.T) {
	a := New(cluster.NewSession())
	a.current = "dev"
	a.router.Push("/workloads")
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	a.onWatch(cluster.WatchEvent{Type: "add", Kind: "Deployment", Object: deployment})
	deployment.Status.ReadyReplicas = 2

	tt := ui.NewTester(a.view, 1280, 900)
	for _, want := range []string{"Overall readiness", "Health by kind", "Inventory", "Deployment", "web", "Active"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q, texts %q", want, tt.Texts())
		}
	}
	stored, ok := a.workloadObjects["Deployment/prod/web"].(*appsv1.Deployment)
	if !ok || stored.Status.ReadyReplicas != 1 {
		t.Fatalf("workload watch stored %#v", stored)
	}
}

func TestWorkloadDetailsUseSeparateShellPanel(t *testing.T) {
	a := New(cluster.NewSession())
	a.current = "dev"
	a.router.Push("/workloads")
	replicas := int32(2)
	a.onWatch(cluster.WatchEvent{Type: "add", Kind: "Deployment", Object: &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	}})

	tt := ui.NewTester(a.view, 1280, 900)
	if tt.HasText("Deployment: web") {
		t.Fatal("details panel opened without a row selection")
	}
	if err := tt.Click("web"); err != nil {
		t.Fatal(err)
	}
	main, mainOK := tt.Find("Main content panel")
	panel, panelOK := tt.Find("Workload details panel")
	if !mainOK || !panelOK {
		t.Fatalf("missing shell panels: main=%v detail=%v", mainOK, panelOK)
	}
	if panel.X-main.X-main.W != shellGap || panel.Y != main.Y || panel.H != main.H {
		t.Fatalf("detail panel is not a separate aligned shell card: main=%+v detail=%+v", main, panel)
	}
	sideHeader, sideHeaderOK := tt.Find("Sidebar header")
	mainHeader, mainHeaderOK := tt.Find("Main header")
	detailHeader, detailHeaderOK := tt.Find("Detail panel header")
	if !sideHeaderOK || !mainHeaderOK || !detailHeaderOK ||
		detailHeader.Y != mainHeader.Y || detailHeader.H != mainHeader.H ||
		detailHeader.Y != sideHeader.Y || detailHeader.H != sideHeader.H {
		t.Fatalf("shell headers are not aligned: side=%+v main=%+v detail=%+v", sideHeader, mainHeader, detailHeader)
	}
	if !tt.HasText("Deployment: web") {
		t.Fatal("selected workload details are missing")
	}
	for _, want := range []string{"Copy name", "Edit", "Remove", "Close details"} {
		if _, ok := tt.Find(want); !ok {
			t.Fatalf("workload detail header missing %q", want)
		}
	}
	if err := tt.Click("Copy name"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "web" {
		t.Fatalf("workload copy wrote %q", got)
	}
	if err := tt.Click("Close details"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("Workload details panel"); ok {
		t.Fatal("close left the details shell panel open")
	}
}

func TestInspectScreensUseSharedShellPanel(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, route, click, panel, detail string
		setup                             func(*App)
	}{
		{
			name: "node", route: "/nodes", click: "cp", panel: "Node details panel", detail: "Test Linux",
			setup: func(a *App) {
				a.onWatch(cluster.WatchEvent{Type: "add", Kind: "Node", Object: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "cp", CreationTimestamp: metav1.NewTime(now.Add(-time.Hour))},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{OSImage: "Test Linux"},
					},
				}})
			},
		},
		{
			name: "namespace", route: "/namespaces", click: "team", panel: "Namespace details panel", detail: "app=shop",
			setup: func(a *App) {
				a.onWatch(cluster.WatchEvent{Type: "add", Kind: "Namespace", Object: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: "team", Labels: map[string]string{"app": "shop"}},
					Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
				}})
			},
		},
		{
			name: "event", route: "/events", click: "Pod/api", panel: "Event details panel", detail: "secret-token was not found",
			setup: func(a *App) {
				a.onWatch(cluster.WatchEvent{Type: "add", Kind: "Event", Object: &corev1.Event{
					ObjectMeta:     metav1.ObjectMeta{Name: "api.1", Namespace: "prod"},
					Type:           corev1.EventTypeWarning,
					Reason:         "Failed",
					Message:        "secret-token was not found",
					InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "api"},
				}})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := New(cluster.NewSession())
			a.current = "dev"
			a.router.Push(test.route)
			test.setup(a)
			tt := ui.NewTester(a.view, 1280, 900)
			if _, ok := tt.Find(test.panel); ok {
				t.Fatal("details panel opened without a row selection")
			}
			if err := tt.Click(test.click); err != nil {
				t.Fatal(err)
			}
			panel, ok := tt.Find(test.panel)
			main, mainOK := tt.Find("Main content panel")
			if !ok || !mainOK || panel.X-main.X-main.W != shellGap || panel.Y != main.Y || panel.H != main.H {
				t.Fatalf("misaligned shared detail panel: main=%+v panel=%+v", main, panel)
			}
			if !tt.HasText(test.detail) {
				t.Fatalf("missing detail %q in %v", test.detail, tt.Texts())
			}
			for _, want := range []string{"Copy name", "Edit", "Remove", "Close details"} {
				if _, ok := tt.Find(want); !ok {
					t.Fatalf("%s header missing %q", test.name, want)
				}
			}
		})
	}
}

func TestGenericResourcesUseSharedShellPanel(t *testing.T) {
	a := New(cluster.NewSession())
	a.row = -1
	snap := listSnap{
		columns: []string{"Namespace", "Name", "Phase"},
		rows: []listRow{{
			Cells:     []string{"prod", "api", "Running"},
			Namespace: "prod",
			Name:      "api",
			Identity:  cluster.Identity{Kind: "Pod", Namespace: "prod", Name: "api"},
		}},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.detailPanel = nil
		t := c.Theme()
		ui.Row(c).Fill().Padding(shellGap).Gap(shellGap).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).FillHeight().Radius(shellRadius).Clip().Background(editorCard(t)).Border(1, t.Border).Label("Main content panel").Children(func() {
				ui.Column(c).Grow(1).Padding(12, 16).Children(func() {
					a.drawList(c, t, "pods", snap)
				})
			})
			a.drawDetailPanel(c, t)
		})
	}, 1000, 700)
	if err := tt.Click("api"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("Pod details panel"); !ok {
		t.Fatalf("generic resource did not open shared panel: %v", tt.Texts())
	}
	for _, want := range []string{"Pod: api", "Namespace", "prod", "Phase", "Running", "Logs", "Exec"} {
		if !tt.HasText(want) {
			t.Fatalf("generic detail missing %q in %v", want, tt.Texts())
		}
	}
	for _, want := range []string{"Copy name", "Edit", "Remove", "Close details"} {
		if _, ok := tt.Find(want); !ok {
			t.Fatalf("generic detail header missing %q", want)
		}
	}
	if err := tt.Click("Copy name"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "api" {
		t.Fatalf("copy wrote %q", got)
	}
	if _, ok := tt.Find("Copied"); !ok {
		t.Fatal("copy action did not show check feedback")
	}
	tt.Advance(2 * time.Second)
	if _, ok := tt.Find("Copy name"); !ok {
		t.Fatal("copy feedback did not reset")
	}
	if err := tt.Click("Remove"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	confirm := a.confirm
	a.mu.Unlock()
	if confirm == nil || confirm.Action != "delete" || confirm.Name != "api" {
		t.Fatalf("remove dispatched %#v", confirm)
	}
}

func TestArgoResourcesUseSharedShellPanel(t *testing.T) {
	a := New(cluster.NewSession())
	a.row = -1
	var action string
	page := argo.Page{
		ID:       "argo-applications",
		Context:  "dev",
		Selected: &a.row,
		Apps: []argo.App{{
			Namespace:   "argo",
			Name:        "shop",
			Sync:        "OutOfSync",
			Health:      "Degraded",
			Destination: "in-cluster/shop",
		}},
		OnAction: func(_ int, name string) { action = name },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.detailPanel = nil
		t := c.Theme()
		ui.Row(c).Fill().Padding(shellGap).Gap(shellGap).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).FillHeight().Radius(shellRadius).Clip().Background(editorCard(t)).Border(1, t.Border).Label("Main content panel").Children(func() {
				ui.Column(c).Grow(1).Padding(12, 16).Children(func() {
					selection := argo.Draw(c, t, page)
					title := argo.DetailTitle(page, selection.Index)
					a.offerDetailPanel(selection.Changed, selection.Valid, shellDetailPanel{
						title:    title,
						label:    "Applications details panel",
						copyText: title,
						onEdit: func() {
							page.OnAction(selection.Index, "yaml")
						},
						onRemove: func() {
							page.OnAction(selection.Index, "delete")
						},
						draw: func(c *ui.Context, t *ui.Theme) {
							argo.Detail(c, t, page, selection.Index)
						},
					})
				})
			})
			a.drawDetailPanel(c, t)
		})
	}, 1000, 700)
	if err := tt.Click("shop"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tt.Find("Applications details panel"); !ok {
		t.Fatalf("Argo resource did not open shared panel: %v", tt.Texts())
	}
	for _, want := range []string{"argo/shop", "OutOfSync", "Degraded", "in-cluster/shop"} {
		if !tt.HasText(want) {
			t.Fatalf("Argo detail missing %q in %v", want, tt.Texts())
		}
	}
	for _, want := range []string{"Copy name", "Edit", "Remove", "Close details"} {
		if _, ok := tt.Find(want); !ok {
			t.Fatalf("Argo detail header missing %q", want)
		}
	}
	if err := tt.Click("Copy name"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "argo/shop" {
		t.Fatalf("copy wrote %q", got)
	}
	if err := tt.Click("Edit"); err != nil {
		t.Fatal(err)
	}
	if action != "yaml" {
		t.Fatalf("edit dispatched %q", action)
	}
	if err := tt.Click("Remove"); err != nil {
		t.Fatal(err)
	}
	if action != "delete" {
		t.Fatalf("remove dispatched %q", action)
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
	if _, ok := tt.Find("NaKube"); ok {
		t.Fatalf("sidebar header still static title, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("No context"); !ok {
		t.Fatalf("missing No context header, texts %q", tt.Texts())
	}
	if !tt.HasText("Overview") || !tt.HasText("Workloads") || !tt.HasText("Pods") {
		t.Fatalf("missing menu, texts %q", tt.Texts())
	}
	if !tt.HasText("metrics unavailable") {
		t.Fatalf("missing metrics card, texts %q", tt.Texts())
	}
}

func TestSideHeaderSelectsContext(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = []string{"dev", "prod"}
	a.current = "dev"
	tt := ui.NewTester(a.view, 1100, 720)
	if _, ok := tt.Find("CONTEXTS"); !ok {
		t.Fatalf("missing contexts section, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("prod"); !ok {
		t.Fatalf("missing context row, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("dev"); !ok {
		t.Fatalf("missing active context, texts %q", tt.Texts())
	}
}

func TestSideHeaderNoList(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = nil
	a.current = "dev"
	tt := ui.NewTester(a.view, 1100, 720)
	if _, ok := tt.Find("dev"); !ok {
		t.Fatalf("missing active context, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("CONTEXTS"); ok {
		t.Fatalf("contexts section drawn without contexts, texts %q", tt.Texts())
	}
}

func TestMenuSearchFilters(t *testing.T) {
	a := New(cluster.NewSession())
	a.menuQuery = "virtual"
	a.contexts = []string{"dev", "prod"}
	a.current = ""
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("VirtualServices") {
		t.Fatalf("missing match, texts %q", tt.Texts())
	}
	if tt.HasText("DaemonSets") || tt.HasText("Workloads") {
		t.Fatalf("unrelated item stayed, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("dev"); !ok {
		t.Fatalf("contexts section filtered by menu search, texts %q", tt.Texts())
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

func TestSwitchContextRefreshesList(t *testing.T) {
	dir := t.TempDir()
	kubeconfig := filepath.Join(dir, "config")
	body := `
apiVersion: v1
kind: Config
current-context: a
contexts:
- name: a
  context:
    cluster: c
    user: u
- name: b
  context:
    cluster: c
    user: u
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:6443
users:
- name: u
  user:
    token: test-token-not-for-logs
`
	if err := os.WriteFile(kubeconfig, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)

	s := cluster.NewSession()
	// Keep Use from writing the picked context into the real per-user state
	// file; the app reads that file at startup.
	s.SetStatePath(filepath.Join(dir, "state", "context"))
	a := New(s)
	a.contexts = []string{"gone-1", "gone-2"}
	a.current = "gone-1"
	a.switchContext("b")
	// switchContext runs Use on a goroutine; poll briefly for completion.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		cur := a.current
		names := append([]string(nil), a.contexts...)
		a.mu.Unlock()
		if cur == "b" {
			if len(names) == 0 {
				t.Fatalf("context list left stale-empty after switch")
			}
			for _, gone := range []string{"gone-1", "gone-2"} {
				for _, name := range names {
					if name == gone {
						t.Fatalf("stale context %q survived switch: %v", gone, names)
					}
				}
			}
			has := func(want string) bool {
				for _, name := range names {
					if name == want {
						return true
					}
				}
				return false
			}
			if !has("a") || !has("b") {
				t.Fatalf("contexts not reloaded from session: %v", names)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("switch did not complete")
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
