package desktop

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/egoist/mygo/ui"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/resource"
)

func TestActionsFollowKind(t *testing.T) {
	pods := resource.Actions("Pod")
	if hasAction(pods, "logs") || hasAction(pods, "exec") || hasAction(pods, "forward") || !hasAction(pods, "yaml") {
		t.Fatalf("generic pod actions must defer runtime container selection to the Pod drawer: %v", pods)
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
	rows := []listRow{
		{Namespace: "ns-a", Name: "one"},
		{Namespace: "ns-b", Name: "two"},
	}
	got := filterListRows(rows, "ns-a")
	if len(got) != 1 || got[0].Namespace != "ns-a" {
		t.Fatalf("filtered rows = %+v", got)
	}
	if got := filterListRows(rows, ""); len(got) != 2 {
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
	if items := a.namespacePaletteItems(); len(items) != 4 || items[0] != allNamespacesLabel || items[1] != "default" {
		t.Fatalf("all palette items = %v", items)
	}
	a.nsPaletteQuery = "all"
	if items := a.namespacePaletteItems(); len(items) != 1 || items[0] != allNamespacesLabel {
		t.Fatalf("all-namespaces palette items = %v", items)
	}
	a.nsPaletteOpen = true
	a.setNamespaceFilter("monitoring")
	if a.nsPaletteOpen {
		t.Fatal("applying a namespace left the palette open")
	}
}

func TestNamespaceSelectOptions(t *testing.T) {
	a := New(cluster.NewSession())
	a.namespaces = map[string]*corev1.Namespace{
		"monitoring":  {ObjectMeta: metav1.ObjectMeta{Name: "monitoring"}},
		"kube-system": {ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	}
	opts := a.namespaceSelectOptions()
	if len(opts) != 3 || opts[0] != "All namespaces" || opts[1] != "kube-system" {
		t.Fatalf("options = %v", opts)
	}
	if got := a.filterForSelection("All namespaces"); got != "" {
		t.Fatalf("All maps to %q", got)
	}
	if got := a.filterForSelection("monitoring"); got != "monitoring" {
		t.Fatalf("selection maps to %q", got)
	}
	if got := a.selectionForFilter(""); got != "All namespaces" {
		t.Fatalf("empty filter shows %q", got)
	}
	if got := a.selectionForFilter("monitoring"); got != "monitoring" {
		t.Fatalf("filter shows %q", got)
	}
}

func TestLivePodsFilterByNamespace(t *testing.T) {
	a := New(cluster.NewSession())
	a.pods = map[string]*corev1.Pod{
		"ns-a/one": {ObjectMeta: metav1.ObjectMeta{Name: "one", Namespace: "ns-a"}},
		"ns-b/two": {ObjectMeta: metav1.ObjectMeta{Name: "two", Namespace: "ns-b"}},
	}
	a.nsFilter = "ns-a"
	cols, rows := a.liveRowsLocked("pods")
	if len(rows) != 1 || rows[0].Namespace != "ns-a" {
		t.Fatalf("live pods rows = %+v", rows)
	}
	if len(cols) == 0 {
		t.Fatal("columns lost")
	}
	a.nsFilter = ""
	if _, rows := a.liveRowsLocked("pods"); len(rows) != 2 {
		t.Fatalf("all pods rows = %+v", rows)
	}
}

func TestSetNamespaceFilterReloadsCachedLists(t *testing.T) {
	a := New(cluster.NewSession())
	a.showScreen("services", true)
	a.mu.Lock()
	b := a.bodies[a.active]
	gen := b.listGen
	a.mu.Unlock()
	a.setNamespaceFilter("default")
	a.mu.Lock()
	defer a.mu.Unlock()
	if b.listGen != gen+1 {
		t.Fatalf("changing the filter did not reload the list: gen %d -> %d", gen, b.listGen)
	}
	if b.nsFilter != "default" {
		t.Fatalf("reload captured namespace %q", b.nsFilter)
	}
}

func TestNamespaceControlsRespectScreenScope(t *testing.T) {
	for _, id := range []string{"workloads", "pods", "events", "services"} {
		if !namespaceScopedScreen(id) {
			t.Errorf("%s should support namespace filtering", id)
		}
	}
	for _, id := range []string{"overview", "nodes", "namespaces", "helm", "port-forwards", "settings", "catalog", "argo-applications"} {
		if namespaceScopedScreen(id) {
			t.Errorf("%s should not support namespace filtering", id)
		}
	}

	a := New(cluster.NewSession())
	a.namespaces = map[string]*corev1.Namespace{
		"default":     {ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		"kube-system": {ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawMain(c, c.Theme(), "port-forwards", listSnap{})
	}, 800, 600)
	if tt.HasText("Namespace") {
		t.Fatal("port-forwards rendered a namespace selector")
	}

	pods := ui.NewTester(func(c *ui.Context) {
		a.drawMain(c, c.Theme(), "pods", listSnap{})
	}, 800, 600)
	title, titleOK := pods.Find("Pods")
	selector, selectorOK := pods.Find("Namespace")
	if !titleOK || !selectorOK {
		t.Fatalf("pods header missing title or namespace selector: %v", pods.Texts())
	}
	titleCenter := title.Y + title.H/2
	selectorCenter := selector.Y + selector.H/2
	if delta := titleCenter - selectorCenter; delta < -4 || delta > 4 {
		t.Fatalf("namespace selector is not inline with title: title y %.1f selector y %.1f", titleCenter, selectorCenter)
	}
	if gap := selector.X - (title.X + title.W); gap < 0 || gap > 20 {
		t.Fatalf("namespace selector is not beside title: gap %.1f", gap)
	}
}

func TestNamespacePaletteIsLabeledAndCanResetToAll(t *testing.T) {
	a := New(cluster.NewSession())
	a.namespaces = map[string]*corev1.Namespace{
		"default":     {ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		"kube-system": {ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	}
	a.showScreen("pods", true)
	a.nsFilter = "default"
	a.nsPaletteOpen = true
	tt := ui.NewTester(func(c *ui.Context) {
		a.drawNamespacePalette(c, c.Theme())
	}, 800, 600)
	if !tt.Focused("Namespace search") {
		t.Fatal("namespace palette search was not labeled and focused")
	}
	if err := tt.Click(allNamespacesLabel); err != nil {
		t.Fatal(err)
	}
	if a.nsFilter != "" || a.nsPaletteOpen {
		t.Fatalf("All namespaces left filter %q open %v", a.nsFilter, a.nsPaletteOpen)
	}

	a.nsPaletteOpen = true
	a.nsPaletteQuery = "ku"
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	if a.nsFilter != "kube-system" || a.nsPaletteOpen {
		t.Fatalf("Enter selected filter %q open %v", a.nsFilter, a.nsPaletteOpen)
	}
}

func TestNamespaceSelectorReadsAreRaceFree(t *testing.T) {
	a := New(cluster.NewSession())
	a.namespaces = map[string]*corev1.Namespace{
		"default": {ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			a.mu.Lock()
			a.namespaces = map[string]*corev1.Namespace{
				"default":     {ObjectMeta: metav1.ObjectMeta{Name: "default"}},
				"kube-system": {ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
			}
			a.mu.Unlock()
		}
	}()
	query := ""
	for i := 0; i < 200; i++ {
		tt := ui.NewTester(func(c *ui.Context) {
			a.drawNamespaceSelector(c, c.Theme())
			a.nsPaletteOpen = true
			a.drawNamespacePalette(c, c.Theme())
		}, 800, 600)
		_ = tt.HasText("Namespace")
		_ = query
	}
	<-done
}

func TestForwardReservationRejectsDuplicateLocalPort(t *testing.T) {
	a := New(cluster.NewSession())
	request := forwardRequest{Context: "dev", Namespace: "shop", Pod: "web", Remote: 80}
	if err := a.reserveForward(8080, &request); err != nil {
		t.Fatal(err)
	}
	duplicate := forwardRequest{Context: "dev", Namespace: "shop", Pod: "web", Remote: 80}
	if err := a.reserveForward(8080, &duplicate); err == nil || err.Error() != "local port 8080 is already starting a forward" {
		t.Fatalf("duplicate starting error = %v", err)
	}
	a.failForwardStart(8080, request.Token, "")
	a.forwards = []*cluster.Forward{{Local: 8080, Remote: 80, Namespace: "shop", Pod: "web"}}
	active := forwardRequest{Context: "dev", Namespace: "shop", Pod: "web", Remote: 80}
	if err := a.reserveForward(8080, &active); err == nil || err.Error() != "local port 8080 is already being forwarded" {
		t.Fatalf("duplicate active error = %v", err)
	}
}

func TestForwardPortsReportsUsefulRanges(t *testing.T) {
	a := New(cluster.NewSession())
	a.forwardLocal, a.forwardRemote = "invalid", "80"
	if _, _, err := a.forwardPorts(); err == nil || err.Error() != "local port must be between 1 and 65535" {
		t.Fatalf("local validation error = %v", err)
	}
	a.forwardLocal, a.forwardRemote = "8080", "70000"
	if _, _, err := a.forwardPorts(); err == nil || err.Error() != "remote port must be between 1 and 65535" {
		t.Fatalf("remote validation error = %v", err)
	}
}

func TestForwardTableIncludesProcessState(t *testing.T) {
	columns, rows := forwardTable([]*cluster.Forward{{
		Context: "dev", Namespace: "shop", Pod: "web", Local: 8080, Remote: 80,
	}})
	if len(columns) != 6 || columns[0] != "Context" || columns[5] != "Status" {
		t.Fatalf("columns = %v", columns)
	}
	if len(rows) != 1 || len(rows[0].Cells) != 6 || rows[0].Cells[5] != "Active" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestStopContextForwardsCancelsStartsAndStopsListeners(t *testing.T) {
	a := New(cluster.NewSession())
	ctx, cancel := context.WithCancel(context.Background())
	fw := &cluster.Forward{Context: "old", Namespace: "shop", Pod: "web", Local: 8080, Remote: 80}
	a.forwardStarting[8081] = forwardRequest{Context: "old", Cancel: cancel}
	a.forwards = []*cluster.Forward{fw}
	a.forwardFailed["old|shop|web|app|80"] = forwardFailure{Message: "failed"}

	a.deactivateContextForwards("old")

	select {
	case <-ctx.Done():
	default:
		t.Fatal("starting forward was not cancelled")
	}
	if fw.Status() != "Stopping" {
		t.Fatalf("active forward status = %q", fw.Status())
	}
	if len(a.forwardStarting) != 1 || len(a.forwardFailed) != 0 {
		t.Fatalf("reservation released or stale failure retained: starting=%v failed=%v", a.forwardStarting, a.forwardFailed)
	}
}

func TestForwardTableFiltersOtherContexts(t *testing.T) {
	_, rows := forwardTableForContext([]*cluster.Forward{
		{Context: "dev", Namespace: "shop", Pod: "web", Local: 8080, Remote: 80},
		{Context: "prod", Namespace: "shop", Pod: "web", Local: 8081, Remote: 80},
	}, "dev")
	if len(rows) != 1 || rows[0].Cells[0] != "dev" {
		t.Fatalf("filtered rows = %+v", rows)
	}
}

func TestFailedContextSwitchPreservesForwardStateAndBlocksNewStarts(t *testing.T) {
	a := New(cluster.NewSession())
	cancelled, cancel := context.WithCancel(context.Background())
	fw := &cluster.Forward{Context: "", Namespace: "shop", Pod: "web", Local: 8080, Remote: 80}
	request := forwardRequest{Context: "", Namespace: "shop", Pod: "web", Local: 8081, Remote: 80, Cancel: cancel, Token: 7}
	a.forwardStarting[8081] = request
	a.forwards = []*cluster.Forward{fw}
	a.forwardEdit = "draft"
	a.forwardFailed["failure"] = forwardFailure{Local: 8082, Remote: 80, Message: "kept"}
	entered := make(chan struct{})
	release := make(chan struct{})
	a.contextUse = func(context.Context, string) error {
		close(entered)
		<-release
		return errors.New("switch denied")
	}

	a.switchContext("new")
	<-entered
	a.forwardLocal, a.forwardRemote = "9090", "80"
	a.startForwardFor(listRow{Namespace: "shop", Name: "other", Identity: cluster.Identity{Kind: "Pod"}}, "new")
	if a.errText != "wait for the context switch to finish" || len(a.forwardStarting) != 1 {
		t.Fatalf("start during switch err=%q starts=%v", a.errText, a.forwardStarting)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for {
		a.mu.Lock()
		switching := a.podEnvSwitching
		a.mu.Unlock()
		if !switching || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case <-cancelled.Done():
		t.Fatal("failed switch cancelled an owned start")
	default:
	}
	if fw.Status() != "Active" || len(a.forwardStarting) != 1 || len(a.forwardFailed) != 1 || a.forwardEdit != "draft" {
		t.Fatalf("failed switch changed forward state: status=%q starts=%v failed=%v edit=%q",
			fw.Status(), a.forwardStarting, a.forwardFailed, a.forwardEdit)
	}
}

func TestLateCancelledStartCannotReleaseNewReservation(t *testing.T) {
	a := New(cluster.NewSession())
	_, cancel := context.WithCancel(context.Background())
	old := forwardRequest{Context: "old", Cancel: cancel}
	if err := a.reserveForward(8080, &old); err != nil {
		t.Fatal(err)
	}
	a.deactivateContextForwards("old")
	replacement := forwardRequest{Context: "new"}
	if err := a.reserveForward(8080, &replacement); err == nil {
		t.Fatal("replacement reserved the port before the cancelled owner exited")
	}
	if !a.clearForwardStart(8080, old.Token) {
		t.Fatal("old owner did not release its reservation")
	}
	if err := a.reserveForward(8080, &replacement); err != nil {
		t.Fatal(err)
	}
	if a.clearForwardStart(8080, old.Token) {
		t.Fatal("late old cleanup removed the replacement reservation")
	}
	if got := a.forwardStarting[8080].Token; got != replacement.Token {
		t.Fatalf("replacement token = %d, want %d", got, replacement.Token)
	}
}

func TestPhoneProjectsOnlyActiveCurrentContextForwards(t *testing.T) {
	a := New(cluster.NewSession())
	active := &cluster.Forward{Context: "", Namespace: "shop", Pod: "web", Local: 8080, Remote: 80}
	stopping := &cluster.Forward{Context: "", Namespace: "shop", Pod: "api", Local: 8081, Remote: 80}
	stopping.Stop()
	other := &cluster.Forward{Context: "other", Namespace: "shop", Pod: "other", Local: 8082, Remote: 80}
	a.forwards = []*cluster.Forward{active, stopping, other}
	got, err := (Phone{App: a}).PortForwards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Pod != "web" {
		t.Fatalf("phone forwards = %+v", got)
	}
}

func TestPhoneForwardSnapshotUsesCommittedAppContextAtomically(t *testing.T) {
	a := New(cluster.NewSession())
	a.forwards = []*cluster.Forward{
		{Context: "", Namespace: "shop", Pod: "startup", Local: 8079, Remote: 80},
		{Context: "old", Namespace: "shop", Pod: "old", Local: 8080, Remote: 80},
		{Context: "new", Namespace: "shop", Pod: "new", Local: 8081, Remote: 80},
	}
	if got := a.portForwardsSnapshot(); len(got) != 1 || got[0].Pod != "startup" {
		t.Fatalf("empty committed-context snapshot = %+v", got)
	}

	a.mu.Lock()
	a.current = "old"
	a.mu.Unlock()
	if got := a.portForwardsSnapshot(); len(got) != 1 || got[0].Pod != "old" {
		t.Fatalf("old committed-context snapshot = %+v", got)
	}

	// This models Session.Use completing before App commits the new context:
	// the snapshot remains on old until a.current changes under App.mu.
	if got := a.portForwardsSnapshot(); len(got) != 1 || got[0].Pod != "old" {
		t.Fatalf("interleaved pre-commit snapshot = %+v", got)
	}
	a.mu.Lock()
	a.current = "new"
	a.mu.Unlock()
	if got := a.portForwardsSnapshot(); len(got) != 1 || got[0].Pod != "new" {
		t.Fatalf("new committed-context snapshot = %+v", got)
	}
}
