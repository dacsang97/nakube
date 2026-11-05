package desktop

import (
	"context"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"

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
}

// App is the native window state. It adapts one Session. It does not open
// a second Kubernetes client.
type App struct {
	mu sync.Mutex

	session *cluster.Session
	window  *mygo.Window
	redraw  func()

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

	watchCancel context.CancelFunc
}

// New returns an app on session. The window opens before the shell import
// finishes. selected starts on the overview.
func New(session *cluster.Session) *App {
	if session == nil {
		session = cluster.NewSession()
	}
	return &App{
		session:    session,
		selected:   "overview",
		appearance: DefaultAppearance(),
		pods:       map[string]*corev1.Pod{},
		nodes:      map[string]*corev1.Node{},
		events:     map[string]*corev1.Event{},
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
			a.pods[key] = pod
		}
	case "Node":
		if ev.Type == "delete" {
			delete(a.nodes, key)
			return
		}
		if node, ok := ev.Object.(*corev1.Node); ok {
			a.nodes[key] = node
		}
	case "Event":
		if ev.Type == "delete" {
			delete(a.events, key)
			return
		}
		if event, ok := ev.Object.(*corev1.Event); ok {
			a.events[key] = event
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
	selected := a.selected
	appearance := a.appearance
	confirm := a.confirm
	a.mu.Unlock()

	t := c.Theme()
	chosen := current
	ui.Row(c).Fill().Children(func() {
		ui.Sidebar(c, &selected, func() {
			for _, section := range Sections() {
				ui.SidebarSection(c, section.Title, nil, func() {
					for _, item := range section.Items {
						ui.SidebarItem(c, item.ID, nil, item.Label)
					}
				})
			}
		}).Width(232)
		ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
			chosen = a.header(c, t, contexts, current, appearance)
			if errText != "" {
				ui.Text(c, errText).TextColor(t.Danger)
			}
			if selected == "overview" || selected == "" {
				a.cards(c, t)
			} else {
				ui.Text(c, labelFor(selected)).FontSize(22).Bold()
				ui.Text(c, "No objects yet").TextColor(t.TextMuted)
			}
			a.confirmDialog(c, t, confirm)
		})
	})

	if chosen != current {
		a.switchContext(chosen)
	}
	a.mu.Lock()
	a.selected = selected
	a.mu.Unlock()
}

func (a *App) header(c *ui.Context, t *ui.Theme, contexts []string, current string, appearance Appearance) string {
	chosen := current
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Nakube").FontSize(22).Bold()
		if len(contexts) == 0 {
			ui.Text(c, "No contexts").TextColor(t.TextMuted)
		} else {
			ui.Select(c, &chosen, contexts).Width(220)
		}
		label := "Dark"
		if appearance == AppearanceDark {
			label = "Light"
		}
		if ui.Button(c, label).Clicked() {
			a.mu.Lock()
			a.appearance = NextAppearance(a.appearance)
			a.mu.Unlock()
		}
	})
	return chosen
}

func (a *App) cards(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "Overview").FontSize(28).Bold()
	ui.Row(c).Gap(12).Children(func() {
		a.card(c, t, "Nodes", a.shown.NodesText())
		a.card(c, t, "Pods", a.shown.PodsText())
		a.card(c, t, "Warnings", a.shown.WarningsText())
		a.card(c, t, "CPU", a.shown.CPU)
		a.card(c, t, "Memory", a.shown.Memory)
	})
}

func (a *App) card(c *ui.Context, t *ui.Theme, title, value string) {
	ui.Column(c).Padding(16).Gap(6).Radius(16).Background(t.Surface).Border(1, t.Border).Width(168).Children(func() {
		ui.Text(c, title).TextColor(t.TextMuted)
		ui.Text(c, value).FontSize(28).Bold()
	})
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
