package desktop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/overview"
)

// Confirm is a dialog's captured identity. Later watch updates do not
// retarget it.
type Confirm struct {
	Open         bool
	Kind         string
	Namespace    string
	Name         string
	Action       string
	GVR          schema.GroupVersionResource
	Replicas     int32
	Revision     int64
	SyncRevision string
	Cascade      string
	Prune        bool
	Targets      []syncTarget
}

// syncTarget is one Application captured for a fleet sync or refresh.
type syncTarget struct {
	Namespace string
	Name      string
	Revision  string
}

// App is the native window state. It adapts one Session. It does not open
// a second Kubernetes client.
type App struct {
	mu sync.Mutex

	session *cluster.Session
	window  *mygo.Window
	redraw  func()
	router  *ui.Router
	tabs    *tabStrip
	active  string
	bodies  map[string]*tabBody
	dragID  string
	dragAcc float32

	selected   string
	appearance Appearance
	contexts   []string
	current    string
	errText    string

	pods            map[string]*corev1.Pod
	nodes           map[string]*corev1.Node
	namespaces      map[string]*corev1.Namespace
	events          map[string]*corev1.Event
	workloadObjects map[string]runtime.Object
	usage           cluster.Usage
	usageErr        error
	samples         []overview.Sample
	seen            map[string]bool
	watchSettled    bool
	metricsSeen     bool

	overviewRole   string
	overviewMetric string
	overviewRange  string

	confirm *Confirm

	mode         string
	row          int
	logLines     []LogLine
	logFilter    string
	logFollow    bool
	logPod       string
	logGen       int
	logLoading   bool
	logCancel    context.CancelFunc
	execTerm     *terminal.Terminal
	execConn     io.Closer
	execErr      string
	yaml         *yamlScreen
	forwards     []*cluster.Forward
	secretReveal bool

	scaleTo       string
	forwardLocal  string
	forwardRemote string
	rollbackRev   string

	helmRelease   string
	helmChart     string
	helmNamespace string
	helmValues    *editor.State
	helmDiff      *editor.DiffState
	helmBusy      bool
	helmDry       bool
	helmErr       string

	syncRev     string
	cascadeMode string
	pruneSync   bool

	argoApps          []cluster.ArgoApp
	argoSets          []cluster.ArgoApplicationSet
	argoProjects      []cluster.ArgoProject
	argoRepos         []cluster.ArgoRepository
	argoClusters      []cluster.ArgoCluster
	argoNotice        string
	argoErr           string
	argoRepoErr       string
	argoClusterErr    string
	argoAppsReady     bool
	argoSetsReady     bool
	argoProjectsReady bool
	argoReposReady    bool
	argoClustersReady bool

	listGen           int
	listID            string
	listLoading       bool
	listErr           string
	listNotice        string
	listCols          []string
	listRows          []listRow
	listQuery         string
	listFilter        int
	listSort          ui.SortOrder
	listColumns       ui.TableLayout
	listKey           string
	listState         ui.ListState
	detailPanel       *shellDetailPanel
	detailPanelOpen   bool
	detailCopiedKey   string
	detailCopiedUntil time.Time
	catalogLoaded     bool
	catalogSynced     time.Time
	catalogPins       []string
	catalogFiles      []string
	catalogErr        string
	addPath           string
	paste             *editor.State

	sidebarWidth float32
	sideDrag     float32
	menuQuery    string

	watchCancel context.CancelFunc
}

// New returns an app on session. The window opens before the shell import
// finishes. selected starts on the overview.
func New(session *cluster.Session) *App {
	if session == nil {
		session = cluster.NewSession()
	}
	return &App{
		session:         session,
		router:          ui.NewRouter("/overview"),
		selected:        "overview",
		appearance:      DefaultAppearance(),
		pods:            map[string]*corev1.Pod{},
		nodes:           map[string]*corev1.Node{},
		namespaces:      map[string]*corev1.Namespace{},
		events:          map[string]*corev1.Event{},
		workloadObjects: map[string]runtime.Object{},
		row:             -1,
		scaleTo:         "1",
		forwardLocal:    "8080",
		forwardRemote:   "80",
		rollbackRev:     "1",
		helmNamespace:   "default",
		overviewRole:    overview.Worker,
		overviewMetric:  overview.CPU,
		overviewRange:   "15m",
	}
}

// Attach stores the window used to coalesce redraws. Tests leave it nil.
func (a *App) Attach(w *mygo.Window) { a.window = w }

// Session is the single cluster client.
func (a *App) Session() *cluster.Session { return a.session }

// Selected is the sidebar id.
func (a *App) Selected() string { a.mu.Lock(); defer a.mu.Unlock(); return a.selected }

// Appearance is light or dark. The layout does not change with it.
func (a *App) Appearance() Appearance {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appearance
}

// ConfirmIdentity is the object the open dialog captured, if any.
func (a *App) ConfirmIdentity() (Confirm, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.confirm == nil || !a.confirm.Open {
		return Confirm{}, false
	}
	return *a.confirm, true
}

// OpenConfirm captures kind, namespace, and name at open time.
func (a *App) OpenConfirm(kind, namespace, name, action string) {
	a.mu.Lock()
	a.confirm = &Confirm{Open: true, Kind: kind, Namespace: namespace, Name: name, Action: action}
	a.mu.Unlock()
	a.invalidate()
}

// Bootstrap imports the login shell without gating the first frame, then
// opens the saved context. A failure is a non-blocking error string.
func (a *App) Bootstrap(ctx context.Context) {
	if err := cluster.ImportShellEnv(); err != nil {
		a.setErr(publicError(err))
	}
	a.session.RefreshLoadingRules()
	a.ensureCatalog()
	a.syncCatalog()
	a.startCatalogWatch(ctx)
	if err := a.session.Open(ctx); err != nil {
		a.setErr(publicError(err))
		a.loadContexts()
		a.invalidate()
		return
	}
	a.loadContexts()
	a.startWatch(ctx)
	a.refreshMetrics(ctx)
	a.invalidate()
}

func (a *App) loadContexts() {
	names, _, err := a.session.Contexts()
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.errText = publicError(err)
		a.contexts = nil
		return
	}
	a.contexts = names
	a.current = a.session.Current()
}

func (a *App) startWatch(ctx context.Context) {
	wctx, cancel := context.WithCancel(ctx)
	a.mu.Lock()
	if a.watchCancel != nil {
		a.watchCancel()
	}
	a.watchCancel = cancel
	a.pods = map[string]*corev1.Pod{}
	a.nodes = map[string]*corev1.Node{}
	a.namespaces = map[string]*corev1.Namespace{}
	a.events = map[string]*corev1.Event{}
	a.workloadObjects = map[string]runtime.Object{}
	a.samples = nil
	a.usage = cluster.Usage{}
	a.usageErr = nil
	a.seen = map[string]bool{}
	a.watchSettled = false
	a.metricsSeen = false
	a.mu.Unlock()
	a.invalidate()
	if err := a.session.Watch(wctx, a.onWatch); err != nil {
		a.setErr(publicError(err))
		a.markListed()
		return
	}
	go a.settleWatch(wctx)
}

func (a *App) refreshMetrics(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			call, cancel := context.WithTimeout(ctx, 12*time.Second)
			usage, err := a.session.Usage(call)
			cancel()
			a.noteUsage(usage, err)
			a.invalidate()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// noteUsage records a metrics-client result. A missing API does not append
// a sample, so the chart cannot invent a zero.
func (a *App) noteUsage(usage cluster.Usage, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.usage = usage
	a.usageErr = err
	a.metricsSeen = true
	if err != nil || !usage.Available {
		return
	}
	a.samples = append(a.samples, overview.Sample{
		At:    time.Now(),
		Nodes: append([]cluster.NodeUsage(nil), usage.Nodes...),
	})
	a.samples = overview.Trim(a.samples, time.Now())
}

func (a *App) onWatch(ev cluster.WatchEvent) {
	a.mu.Lock()
	if a.seen == nil {
		a.seen = map[string]bool{}
	}
	a.seen[ev.Kind] = true
	a.fold(ev)
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) fold(ev cluster.WatchEvent) {
	key := objectKey(ev.Object)
	a.foldWorkload(ev, key)
	switch ev.Kind {
	case "Pod":
		if ev.Type == "delete" {
			delete(a.pods, key)
			return
		}
		if pod, ok := ev.Object.(*corev1.Pod); ok {
			a.pods[key] = pod.DeepCopy()
		}
	case "Node":
		if ev.Type == "delete" {
			delete(a.nodes, key)
			return
		}
		if node, ok := ev.Object.(*corev1.Node); ok {
			a.nodes[key] = node.DeepCopy()
		}
	case "Namespace":
		if a.namespaces == nil {
			a.namespaces = map[string]*corev1.Namespace{}
		}
		if ev.Type == "delete" {
			delete(a.namespaces, key)
			return
		}
		if item, ok := ev.Object.(*corev1.Namespace); ok {
			a.namespaces[key] = item.DeepCopy()
		}
	case "Event":
		if ev.Type == "delete" {
			delete(a.events, key)
			return
		}
		if event, ok := ev.Object.(*corev1.Event); ok {
			a.events[key] = event.DeepCopy()
		}
	}
}

func (a *App) foldWorkload(ev cluster.WatchEvent, key string) {
	switch ev.Kind {
	case "Pod", "Deployment", "ReplicaSet", "DaemonSet", "StatefulSet", "Job", "CronJob":
	default:
		return
	}
	if a.workloadObjects == nil {
		a.workloadObjects = map[string]runtime.Object{}
	}
	key = ev.Kind + "/" + key
	if ev.Type == "delete" {
		delete(a.workloadObjects, key)
		return
	}
	if ev.Object != nil {
		a.workloadObjects[key] = ev.Object.DeepCopyObject()
	}
}

func (a *App) invalidate() {
	if a.redraw != nil {
		a.redraw()
		return
	}
	if a.window != nil {
		a.window.Invalidate()
	}
}

func (a *App) setErr(msg string) {
	a.mu.Lock()
	a.errText = msg
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) switchContext(name string) {
	if name == "" || name == a.session.Current() {
		return
	}
	go func() {
		if err := a.session.Use(context.Background(), name); err != nil {
			a.setErr(publicError(err))
			return
		}
		a.mu.Lock()
		a.current = name
		a.errText = ""
		a.mu.Unlock()
		a.loadContexts()
		a.startWatch(context.Background())
		a.refreshMetrics(context.Background())
		a.loadList(a.Selected())
		a.invalidate()
	}()
}

// View draws the window. It applies the coalesced snapshot on the frame.
func (a *App) View(c *ui.Context) { a.view(c) }

func (a *App) view(c *ui.Context) {
	a.mu.Lock()
	wait := overview.Wait{}
	if a.errText == "" {
		wait.Nodes = !a.kindReady("Node")
		wait.Pods = !a.kindReady("Pod")
		wait.Events = !a.kindReady("Event")
		wait.Facts = wait.Nodes && wait.Pods && wait.Events
		wait.Metrics = !a.metricsSeen
	}
	snap := overview.From(a.nodes, a.pods, a.events, a.usage, a.usageErr, time.Now())
	samples := append([]overview.Sample(nil), a.samples...)
	current := a.current
	contexts := a.contexts
	errText := a.errText
	confirm := a.confirm
	a.mu.Unlock()
	a.detailPanel = nil
	a.followRouter()

	t := c.Theme()
	shell := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(shellBackdrop(t)).Padding(shellGap).Gap(shellGap)
	shell.Children(func() {
		ui.Row(c).Shrink(0).FillHeight().Radius(shellRadius).Clip().Background(t.Surface).Border(1, t.Border).Children(func() {
			a.drawActivityBar(c, t, current)
			a.drawSide(c, t, current, contexts)
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Radius(shellRadius).Clip().Background(editorCard(t)).Border(1, t.Border).Label("Main content panel").Children(func() {
			a.drawTabs(c, t)
			ui.Column(c).Grow(1).MinWidth(0).FillWidth().Padding(12, 16).Gap(12).Children(func() {
				if errText != "" {
					ui.Text(c, errText).TextColor(t.Danger)
				}
				a.router.View(c, func(r *ui.Route) {
					a.drawPage(c, t, r, current, confirm, snap, samples, wait)
				})
			})
		})
		a.drawDetailPanel(c, t)
	})
	a.tabKeys(shell)
}

func (a *App) drawPage(c *ui.Context, t *ui.Theme, r *ui.Route, current string, confirm *Confirm, shown overview.Snapshot, samples []overview.Sample, wait overview.Wait) {
	if d := a.activeDetail(); d != nil {
		r.Title(d.title)
		switch d.kind {
		case "logs":
			a.drawLogs(c, t)
		case "exec":
			a.drawExec(c, t)
		case "yaml":
			a.drawYAML(c, t)
		}
		a.confirmDialog(c, t, confirm)
		return
	}
	switch {
	case r.Match("/catalog"):
		r.Title("Catalog")
		a.drawCatalog(c, t, current)
	case r.Match("/settings"):
		r.Title("Settings")
		a.drawSettings(c, t)
	case r.Match("/overview"), r.Match("/"):
		r.Title("Overview")
		overview.Draw(c, t, shown, samples, current, &a.overviewRole, &a.overviewMetric, &a.overviewRange, time.Now(), a.openPage, &wait)
	case r.Match("/{id}"):
		id := r.Param("id")
		r.Title(labelFor(id))
		if inspectScreen(id) {
			a.drawInspect(c, t, id)
			break
		}
		a.mu.Lock()
		list := a.snapshotLocked(id)
		if a.errText == "" {
			if kind := watchKind(id); kind != "" && !a.kindReady(kind) && len(list.rows) == 0 {
				list.loading = true
			}
		}
		a.mu.Unlock()
		a.drawMain(c, t, id, list)
	default:
		r.Title("Not found")
		ui.Text(c, "Not found")
	}
	a.confirmDialog(c, t, confirm)
}

func (a *App) kindReady(kind string) bool {
	if a.watchSettled || a.seen[kind] || a.session.KindSynced(kind) {
		return true
	}
	switch kind {
	case "Pod":
		return len(a.pods) > 0
	case "Node":
		return len(a.nodes) > 0
	case "Namespace":
		return len(a.namespaces) > 0
	case "Event":
		return len(a.events) > 0
	default:
		return false
	}
}

func (a *App) settleWatch(ctx context.Context) {
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		if a.session.KindSynced("Pod") && a.session.KindSynced("Node") && a.session.KindSynced("Namespace") && a.session.KindSynced("Event") {
			a.markListed()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			a.markListed()
			return
		case <-tick.C:
		}
	}
}

func (a *App) markListed() {
	a.mu.Lock()
	a.watchSettled = true
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) openPage(id string) {
	a.showScreen(id, true)
}

func (a *App) togglePage(path string) {
	a.ensureTabs()
	id := screenID(path)
	if a.tabs.Screen(a.active) == id {
		a.closeTab(a.active)
		return
	}
	a.showScreen(id, true)
}

func (a *App) closeCatalog() {
	a.ensureTabs()
	if id, ok := a.tabs.byScreen("catalog"); ok {
		a.closeTab(id)
	}
}

func pagePath(id string) string {
	id = strings.Trim(strings.TrimSpace(id), "/")
	if id == "" {
		return "/overview"
	}
	return "/" + id
}

func screenID(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if i := strings.IndexByte(path, '/'); i >= 0 {
		path = path[:i]
	}
	if path == "" {
		return "overview"
	}
	return path
}

func (a *App) confirmDialog(c *ui.Context, t *ui.Theme, confirm *Confirm) {
	if confirm == nil || !confirm.Open {
		return
	}
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Padding(16).Gap(8).Children(func() {
			ui.Text(c, confirm.Action+" "+confirm.Kind).FontSize(18).Bold()
			ui.Text(c, confirm.Namespace+"/"+confirm.Name)
			if extra := confirmExtra(confirm); extra != "" {
				ui.Text(c, extra).TextColor(t.TextMuted)
			}
			ui.Text(c, "This dialog keeps the object it opened on.").TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Cancel").Clicked() {
					open = false
				}
				if ui.Button(c, "Confirm").Clicked() {
					a.applyConfirm()
					open = false
				}
			})
		})
	})
	if !open {
		a.mu.Lock()
		if a.confirm != nil {
			a.confirm.Open = false
		}
		a.mu.Unlock()
	}
}

func confirmExtra(confirm *Confirm) string {
	if confirm == nil {
		return ""
	}
	switch confirm.Action {
	case "delete":
		if confirm.Kind != "Application" {
			return ""
		}
		mode := confirm.Cascade
		if mode == "" {
			mode = "foreground"
		}
		return "Cascade: " + mode
	case "sync":
		rev := confirm.SyncRevision
		if rev == "" {
			rev = "HEAD"
		}
		line := "Revision: " + rev
		if confirm.Prune {
			line += " · prune"
		}
		return line
	case "sync-drift":
		line := fmt.Sprintf("%d applications", len(confirm.Targets))
		if confirm.Prune {
			line += " · prune"
		}
		return line
	case "refresh-all":
		return fmt.Sprintf("%d applications", len(confirm.Targets))
	case "hard-refresh":
		return "Hard refresh drops the manifest cache"
	default:
		return ""
	}
}

func labelFor(id string) string {
	for _, section := range Sections() {
		for _, item := range section.Items {
			if item.ID == id {
				return item.Label
			}
		}
	}
	return id
}

func objectKey(obj any) string {
	type named interface {
		GetNamespace() string
		GetName() string
	}
	n, ok := obj.(named)
	if !ok || n == nil {
		return ""
	}
	if ns := n.GetNamespace(); ns != "" {
		return ns + "/" + n.GetName()
	}
	return n.GetName()
}

func publicError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "token") || strings.Contains(lower, "certificate-authority-data") || strings.Contains(lower, "kubeconfig file") {
		return "could not connect to the cluster"
	}
	return msg
}
