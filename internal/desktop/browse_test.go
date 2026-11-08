package desktop

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/resource"
)

func TestActionsFollowKind(t *testing.T) {
	pods := resource.Actions("Pod")
	if !hasAction(pods, "logs") || !hasAction(pods, "exec") || hasAction(pods, "sync") {
		t.Fatalf("pod actions = %v", pods)
	}
	deps := resource.Actions("Deployment")
	if !hasAction(deps, "scale") || !hasAction(deps, "restart") || hasAction(deps, "logs") {
		t.Fatalf("deployment actions = %v", deps)
	}
	if resource.Actions("") != nil {
		t.Fatal("summary rows should have no actions")
	}
}

func TestPodTableUsesTheWatch(t *testing.T) {
	a := New(nil)
	a.pods["ns/web"] = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	_, rows := a.liveRowsLocked("pods")
	if len(rows) != 1 || rows[0].Name != "web" || rows[0].Identity.Kind != "Pod" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Cells[2] != "Running" {
		t.Fatalf("phase = %q", rows[0].Cells[2])
	}
}

func TestAskCapturesReplicas(t *testing.T) {
	a := New(nil)
	a.scaleTo = "3"
	a.ask(listRow{Namespace: "ns", Name: "web", Identity: cluster.Identity{Kind: "Deployment"}}, "scale")
	got, ok := a.ConfirmIdentity()
	if !ok || got.Replicas != 3 || got.Name != "web" {
		t.Fatalf("confirm = %+v ok=%v", got, ok)
	}
	a.scaleTo = "nope"
	a.ask(listRow{Name: "web", Identity: cluster.Identity{Kind: "Deployment"}}, "scale")
	if a.errText == "" {
		t.Fatal("bad replica count was accepted")
	}
}

func TestClusterScreensStayLive(t *testing.T) {
	for _, id := range []string{"nodes", "namespaces", "events"} {
		if !isLive(id) || watchKind(id) == "" {
			t.Fatalf("%s fell back to a fetched name list", id)
		}
	}
}

func TestFoldKeepsNamespacePhase(t *testing.T) {
	a := New(nil)
	a.fold(cluster.WatchEvent{
		Type: "add",
		Kind: "Namespace",
		Object: &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "team", Labels: map[string]string{"app": "shop"}},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		},
	})
	got := a.namespaces["team"]
	if got == nil || got.Status.Phase != corev1.NamespaceActive || got.Labels["app"] != "shop" {
		t.Fatalf("%+v", got)
	}
	a.fold(cluster.WatchEvent{Type: "delete", Kind: "Namespace", Object: got})
	if _, ok := a.namespaces["team"]; ok {
		t.Fatal("deleted namespace stayed in the watch")
	}
}

func hasAction(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func TestFilterListRows(t *testing.T) {
	a := New(cluster.NewSession())
	rows := []listRow{
		{Namespace: "ns-a", Name: "one"},
		{Namespace: "ns-b", Name: "two"},
	}
	a.nsFilter = "ns-a"
	got := a.filterListRows(rows)
	if len(got) != 1 || got[0].Namespace != "ns-a" {
		t.Fatalf("filtered rows = %+v", got)
	}
	a.nsFilter = ""
	if got := a.filterListRows(rows); len(got) != 2 {
		t.Fatalf("all rows = %+v", got)
	}
}

func TestSelectNamespaceResetsRow(t *testing.T) {
	a := New(cluster.NewSession())
	a.namespaces = map[string]*corev1.Namespace{
		"default":     {ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		"kube-system": {ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	}
	a.row = 3
	a.setNamespaceFilter("kube-system")
	if a.nsFilter != "kube-system" || a.row != -1 {
		t.Fatalf("filter %q row %d", a.nsFilter, a.row)
	}
	a.setNamespaceFilter("")
	if a.nsFilter != "" {
		t.Fatalf("all filter = %q", a.nsFilter)
	}
}

func TestNamespacePaletteFilter(t *testing.T) {
	a := New(cluster.NewSession())
	a.namespaces = map[string]*corev1.Namespace{
		"monitoring":  {ObjectMeta: metav1.ObjectMeta{Name: "monitoring"}},
		"kube-system": {ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		"default":     {ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	}
	a.nsPaletteQuery = "ku"
	items := a.namespacePaletteItems()
	if len(items) != 1 || items[0] != "kube-system" {
		t.Fatalf("palette items = %v", items)
	}
	a.nsPaletteQuery = ""
	if items := a.namespacePaletteItems(); len(items) != 3 || items[0] != "default" {
		t.Fatalf("all palette items = %v", items)
	}
	a.nsPaletteOpen = true
	a.setNamespaceFilter("monitoring")
	if a.nsPaletteOpen {
		t.Fatal("applying a namespace left the palette open")
	}
}
