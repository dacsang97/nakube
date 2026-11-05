package desktop

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
)

// Confirm is a dialog's captured identity. Later watch updates do not
// retarget it.
type Confirm struct {
	Open      bool
	Kind      string
	Namespace string
	Name      string
	Action    string
	GVR       schema.GroupVersionResource
	Replicas  int32
	Revision  int64
}

// App is the native window state. It adapts one Session. It does not open
// a second Kubernetes client.
type App struct {
	mu sync.Mutex

	session *cluster.Session
	window  *mygo.Window
	redraw  func()
	router  *ui.Router

	selected   string
	appearance Appearance
	contexts   []string
	current    string
	errText    string

	pods     map[string]*corev1.Pod
	nodes    map[string]*corev1.Node
	events   map[string]*corev1.Event
	usage    cluster.Usage
	usageErr error

	shown   Overview
	confirm *Confirm

	mode         string
	row          int
	logLines     []LogLine
	logFilter    string
	logFollow    bool
	logPod       string
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
	helmDry       bool
	helmErr       string

	listGen     int
	listID      string
	listLoading bool
	listErr     string
	listNotice  string
	listCols    []string
	listRows    []listRow

	catalogLoaded bool
	catalogSynced time.Time
	catalogPins   []string
	catalogFiles  []string
	catalogErr    string
	addPath       string
	paste         *editor.State

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
		session:       session,
		router:        ui.NewRouter("/overview"),
		selected:      "overview",
		appearance:    DefaultAppearance(),
		pods:          map[string]*corev1.Pod{},
		nodes:         map[string]*corev1.Node{},
		events:        map[string]*corev1.Event{},
		scaleTo:       "1",
		forwardLocal:  "8080",
		forwardRemote: "80",
		rollbackRev:   "1",
		helmNamespace: "default",
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
	a.events = map[string]*corev1.Event{}
	a.mu.Unlock()
	if err := a.session.Watch(wctx, a.onWatch); err != nil {
		a.setErr(publicError(err))
	}
}

func (a *App) refreshMetrics(ctx context.Context) {
	go func() {
		usage, err := a.session.Usage(ctx)
		a.noteUsage(usage, err)
		a.invalidate()
	}()
}

// noteUsage records a metrics-client result. NotFound becomes the
// unavailable card text and does not invent a number.
func (a *App) noteUsage(usage cluster.Usage, err error) {
	a.mu.Lock()
	a.usage = usage
	a.usageErr = err
	a.mu.Unlock()
}

func (a *App) onWatch(ev cluster.WatchEvent) {
	a.mu.Lock()
	a.fold(ev)
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) fold(ev cluster.WatchEvent) {
	key := objectKey(ev.Object)
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
	a.shown = overviewFrom(a.nodes, a.pods, a.events, a.usage, a.usageErr)
	contexts := append([]string(nil), a.contexts...)
	current := a.current
	errText := a.errText
	prev := a.selected
	confirm := a.confirm
	a.mu.Unlock()
	if a.router == nil {
		a.router = ui.NewRouter("/overview")
	}

	t := c.Theme()
	chosen := current
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(shellBackdrop(t)).Padding(shellGap).Gap(shellGap).Children(func() {
		ui.Row(c).Shrink(0).FillHeight().Radius(shellRadius).Clip().Background(t.Surface).Border(1, t.Border).Children(func() {
			a.drawActivityBar(c, t, current)
			a.drawSide(c, t)
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Radius(shellRadius).Clip().Background(editorCard(t)).Border(1, t.Border).Padding(12, 16).Gap(12).Children(func() {
			chosen = a.header(c, t, contexts, current)
			if errText != "" {
				ui.Text(c, errText).TextColor(t.Danger)
			}
			a.router.View(c, func(r *ui.Route) {
				a.drawPage(c, t, r, current, confirm)
			})
		})
	})

	if chosen != current {
		a.switchContext(chosen)
	}
	id := screenID(a.router.Path())
	if id == prev {
		return
	}
	a.row = 0
	a.leaveOverlay()
	a.mu.Lock()
	a.selected = id
	a.mu.Unlock()
	a.loadList(id)
}

func (a *App) drawPage(c *ui.Context, t *ui.Theme, r *ui.Route, current string, confirm *Confirm) {
	switch {
	case r.Match("/catalog"):
		r.Title("Catalog")
		a.drawCatalog(c, t, current)
	case r.Match("/settings"):
		r.Title("Settings")
		a.drawSettings(c, t)
	case r.Match("/overview"), r.Match("/"):
		r.Title("Overview")
		a.cards(c, t)
	case r.Match("/{id}"):
		id := r.Param("id")
		r.Title(labelFor(id))
		a.mu.Lock()
		snap := a.snapshotLocked(id)
		a.mu.Unlock()
		a.drawMain(c, t, id, snap)
	default:
		r.Title("Not found")
		ui.Text(c, "Not found")
	}
	a.confirmDialog(c, t, confirm)
}

func (a *App) openPage(id string) {
	if a.router == nil {
		return
	}
	a.router.Push(pagePath(id))
}

func (a *App) togglePage(path string) {
	if a.router == nil {
		return
	}
	if a.router.Path() == path {
		a.router.Back()
		if a.router.Path() == path {
			a.router.Replace("/overview")
		}
		return
	}
	a.router.Push(path)
}

func (a *App) closeCatalog() {
	if a.router == nil || a.router.Path() != "/catalog" {
		return
	}
	a.router.Back()
	if a.router.Path() == "/catalog" {
		a.router.Replace("/overview")
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

func (a *App) header(c *ui.Context, t *ui.Theme, contexts []string, current string) string {
	chosen := current
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "NaKube").FontSize(22).Bold()
		if len(contexts) == 0 {
			ui.Text(c, "No contexts").TextColor(t.TextMuted)
		} else {
			ui.Select(c, &chosen, contexts).Width(220)
		}
	})
	return chosen
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
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "token") || strings.Contains(lower, "certificate-authority-data") || strings.Contains(lower, "kubeconfig file") {
		return "could not connect to the cluster"
	}
	return msg
}
