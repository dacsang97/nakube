package desktop

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
	"github.com/dacsang97/nakube/internal/views/pods"
)

// podUsageEvery is how often a shown Pods list reads metrics-server, as
// the node sample does.
const podUsageEvery = 15 * time.Second

// podUsageFetcher reads pod metrics. Tests replace it.
type podUsageFetcher func(ctx context.Context, fallback []string) (map[string]cluster.PodUsage, bool, error)

// podUsageSpread caps the namespaces read one by one when the account may
// not list pod metrics cluster wide.
const podUsageSpread = 30

func (a *App) drawPods(c *ui.Context, t *ui.Theme) {
	now := time.Now()
	a.mu.Lock()
	items := make([]*corev1.Pod, 0, len(a.pods))
	for _, p := range a.pods {
		if p != nil && (a.nsFilter == "" || p.Namespace == a.nsFilter) {
			items = append(items, p)
		}
	}
	usage := a.podUsage
	loading := a.errText == "" && !a.kindReady("Pod") && len(items) == 0
	a.ensurePodUsageLocked(now, podNamespaces(items))
	a.mu.Unlock()

	a.prepareInspect("pods")
	page := pods.Build(items, usage, a.listQuery, a.listSort.Column, a.listSort.Descending)
	keys := make([]string, len(page.Rows))
	for i, row := range page.Rows {
		keys[i] = row.Key
	}
	a.bindList(keys)
	// A link opens its screen after this list has stored its own state, so
	// the other tab's selection is not written over.
	var openScreen, openKey string
	selection := pods.Draw(c, t, page, &a.listQuery, &a.listState, loading, now, func(screen, key string) {
		openScreen, openKey = screen, key
	}, a.drawNamespaceSelector)
	if selection.Valid {
		row := page.Rows[selection.Index]
		lr := listRow{Namespace: row.Namespace, Name: row.Name, Identity: row.Identity}
		a.mu.Lock()
		pod := a.pods[row.Key]
		if pod != nil {
			pod = pod.DeepCopy()
		}
		target, envEditable := podEnvironmentTarget(pod, a.workloadObjects)
		a.mu.Unlock()
		a.bindPodEnvironment(row.Key)
		onEdit, onRemove := a.panelResourceActions(lr)
		containerNames := make([]string, 0)
		if pod != nil {
			containerNames = make([]string, 0, len(pod.Spec.Containers))
			for _, container := range pod.Spec.Containers {
				containerNames = append(containerNames, container.Name)
			}
		}
		namespace, podName := row.Namespace, row.Name
		a.offerDetailPanel(selection.Changed, true, shellDetailPanel{
			title:       "Pod: " + row.Name,
			label:       "Pod details panel",
			copyText:    row.Name,
			editLabel:   "Edit YAML",
			removeLabel: "Delete",
			onEdit:      onEdit,
			onRemove:    onRemove,
			drawHeaderActions: func(c *ui.Context, t *ui.Theme) {
				a.drawPodHeaderAction(c, t, "Logs", "list", containerNames, func(container string) {
					a.openPodLogs(namespace, podName, container)
				})
				a.drawPodHeaderAction(c, t, "Shell", "terminal", containerNames, func(container string) {
					a.openPodShell(namespace, podName, container)
				})
			},
			draw: func(c *ui.Context, t *ui.Theme) {
				a.drawPodMetrics(c, t, row.Namespace, row.Name)
				a.drawPodParameters(c, t, row, lr, pod)
				a.drawPodEnvironment(c, t, pod, target, envEditable)
			},
		})
	} else {
		a.offerDetailPanel(selection.Changed, false, shellDetailPanel{})
	}
	a.finishInspect(keys)
	if openScreen != "" {
		a.openListAt(openScreen, openKey)
	}
}

func (a *App) drawPodParameters(c *ui.Context, t *ui.Theme, row pods.Row, lr listRow, pod *corev1.Pod) {
	owner := "—"
	if row.Owner.Kind != "" {
		owner = row.Owner.Kind + " " + row.Owner.Name
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "Pod parameters").FontSize(14).FontWeight(650)
		ui.Column(c).Gap(8).Label("Pod parameter facts").Children(func() {
			kit.Facts(c, t, []kit.Fact{
				{Label: "Status", Value: row.Status},
				{Label: "Ready", Value: readyDots(row.Dots)},
				{Label: "Restarts", Value: strconv.Itoa(int(row.Restarts))},
				{Label: "Node", Value: row.Node},
				{Label: "Namespace", Value: row.Namespace},
				{Label: "Name", Value: row.Name},
				{Label: "Controlled By", Value: owner},
				{Label: "QoS", Value: row.QoS},
			})
		})
		a.drawPodPortForwards(c, t, pod, lr)
	})
}

func (a *App) drawPodPortForwards(c *ui.Context, t *ui.Theme, pod *corev1.Pod, lr listRow) {
	containers := []corev1.Container(nil)
	if pod != nil {
		containers = pod.Spec.Containers
	}
	ui.Column(c).Gap(8).Padding(10).Radius(6).Background(t.Surface).Border(1, t.Border).
		Label("Pod port forwards").Children(func() {
		ui.Text(c, "Port forwards").FontSize(13).FontWeight(650)
		a.drawPodPorts(c, t, containers, lr)
	})
}

func (a *App) drawPodHeaderAction(
	c *ui.Context,
	t *ui.Theme,
	label, icon string,
	containers []string,
	open func(string),
) {
	button := panelHeaderButton(c, t, label, icon, false).Tooltip("Open " + strings.ToLower(label))
	switch len(containers) {
	case 0:
		button.Disabled(true)
	case 1:
		container := containers[0]
		if button.Clicked() {
			open(container)
		}
	default:
		button.PopoverMenuButton(func(menu *ui.Menu) {
			for _, name := range containers {
				container := name
				if menu.Item(label + ": " + container).Chosen() {
					open(container)
				}
			}
		})
	}
}

func (a *App) openPodLogs(namespace, pod, container string) {
	if a.podLogsOpen != nil {
		a.podLogsOpen(namespace, pod, container)
		return
	}
	a.openLogs(namespace, pod, container)
}

func (a *App) openPodShell(namespace, pod, container string) {
	if container == "" {
		a.setErr("shell requires a container")
		return
	}
	if a.podShellOpen != nil {
		a.podShellOpen(namespace, pod, container)
		return
	}
	a.openExec(namespace, pod, container)
}

type podPortGroup struct {
	remote     uint16
	containers []string
	names      []string
}

func (a *App) drawPodPorts(c *ui.Context, t *ui.Theme, containers []corev1.Container, lr listRow) {
	declared := 0
	var tcp []*podPortGroup
	byRemote := map[uint16]*podPortGroup{}
	type unsupportedPort struct {
		container string
		name      string
		remote    int32
		protocol  corev1.Protocol
	}
	var unsupported []unsupportedPort
	for _, container := range containers {
		for _, port := range container.Ports {
			declared++
			protocol := port.Protocol
			if protocol == "" {
				protocol = corev1.ProtocolTCP
			}
			if protocol != corev1.ProtocolTCP || port.ContainerPort <= 0 || port.ContainerPort > 65535 {
				unsupported = append(unsupported, unsupportedPort{
					container: container.Name, name: port.Name, remote: port.ContainerPort, protocol: protocol,
				})
				continue
			}
			remote := uint16(port.ContainerPort)
			group := byRemote[remote]
			if group == nil {
				group = &podPortGroup{remote: remote}
				byRemote[remote] = group
				tcp = append(tcp, group)
			}
			if !slices.Contains(group.containers, container.Name) {
				group.containers = append(group.containers, container.Name)
			}
			name := port.Name
			if name == "" {
				name = "unnamed"
			}
			if !slices.Contains(group.names, name) {
				group.names = append(group.names, name)
			}
		}
	}
	ui.Text(c, "Ports").FontSize(12).FontWeight(650)
	if declared == 0 {
		ui.Text(c, "No declared container ports.").FontSize(12).TextColor(t.TextMuted)
		key := a.podForwardKey(lr, 0)
		a.drawPodPortRow(c, t, lr, key, "Custom port", "", 0, true)
		return
	}
	for _, group := range tcp {
		title := strings.Join(group.containers, ", ")
		detail := fmt.Sprintf("%s · %d/TCP", strings.Join(group.names, ", "), group.remote)
		key := a.podForwardKey(lr, group.remote)
		a.drawPodPortRow(c.Key(key), t, lr, key, title, detail, group.remote, false)
	}
	for _, port := range unsupported {
		name := port.name
		if name == "" {
			name = "unnamed"
		}
		title := port.container
		detail := fmt.Sprintf("%s · %d/%s", name, port.remote, port.protocol)
		ui.Column(c.Key(fmt.Sprintf("%s-%d-%s", title, port.remote, port.protocol))).Gap(4).Padding(8).
			Radius(4).Background(t.SurfaceHover).Label(title + " unsupported port").Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Column(c).Gap(1).Grow(1).MinWidth(120).Children(func() {
					ui.Text(c, title).FontSize(12).FontWeight(650)
					ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted)
				})
				ui.Badge(c, "Unsupported")
			})
			ui.Text(c, fmt.Sprintf("%s port forwarding is not supported.", port.protocol)).
				FontSize(11).TextColor(t.TextMuted)
		})
	}
}

func (a *App) drawPodPortRow(c *ui.Context, t *ui.Theme, lr listRow, key, title, detail string, remote uint16, custom bool) {
	fw, starting, failure, failed := a.forwardState(key)
	status := ""
	if fw != nil {
		status = fw.Status()
	}
	ui.Column(c).Gap(6).Padding(8).Radius(4).Background(t.SurfaceHover).
		Label(title + " port forward").Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, title).FontSize(12).FontWeight(650).SingleLine().Tooltip(title)
				if detail != "" {
					ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(detail)
				}
			})
			switch {
			case fw != nil && status != "Active":
				ui.Badge(c, status).Shrink(0)
			case starting == nil && a.forwardEdit != key && !failed && fw == nil:
				if ui.Button(c, "Start").MinHeight(32).Shrink(0).
					Label("Start " + title + " port forward").Clicked() {
					a.beginForwardEdit(key, remote, custom)
				}
			}
		})
		if fw != nil && status == "Active" {
			destination := fmt.Sprintf("http://127.0.0.1:%d", fw.Local)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Label(title + " active destination").Children(func() {
				ui.Badge(c, "Active").Shrink(0)
				if ui.Button(c, destination).MinHeight(32).Grow(1).MinWidth(0).
					Background(t.Accent).TextColor(t.AccentText).Tooltip("Open " + destination).
					Label("Open " + destination + " in browser").Clicked() {
					a.openForward(fw.Local)
				}
				if ui.Button(c, "Stop").MinHeight(32).Shrink(0).TextColor(t.Danger).
					Label("Stop " + title + " port forward").Clicked() {
					a.stopForward(fw)
				}
			})
		}
		if starting != nil {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Badge(c, "Starting").Shrink(0)
				ui.Text(c, fmt.Sprintf("127.0.0.1:%d", starting.Local)).FontSize(11).TextColor(t.TextMuted).
					Grow(1).MinWidth(0).SingleLine()
			})
		}
		if failed {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Badge(c, "Failed")
				ui.Text(c, failure.Message).FontSize(11).TextColor(t.Danger).Grow(1).MinWidth(120)
				retry := ui.Button(c, "Retry").MinHeight(32).Label("Retry " + title + " port forward")
				if retry.Clicked() {
					a.forwardLocal = strconv.Itoa(int(failure.Local))
					a.forwardRemote = strconv.Itoa(int(failure.Remote))
					a.startForwardFor(lr, key)
				}
			})
		}
		if a.forwardEdit == key {
			a.drawForwardForm(c, t, lr, key, title, remote, custom)
		}
	})
}

func (a *App) drawForwardForm(c *ui.Context, t *ui.Theme, lr listRow, key, title string, remote uint16, custom bool) {
	ui.Column(c).Gap(6).Label(title + " port forward form").Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
			ui.TextInput(c, &a.forwardLocal).Label("Local port").Width(104)
			if custom {
				ui.TextInput(c, &a.forwardRemote).Label("Remote port").Width(104)
			} else {
				ui.Text(c, fmt.Sprintf("Remote %d", remote)).FontSize(12).TextColor(t.TextMuted)
			}
		})
		ui.Row(c).Gap(6).Wrap().Children(func() {
			if ui.Button(c, "Cancel").MinHeight(32).Clicked() {
				a.forwardEdit = ""
			}
			if ui.Button(c, "Start forwarding").MinHeight(32).Label("Start forwarding " + title).Clicked() {
				if _, _, err := a.forwardPorts(); err != nil {
					a.setErr(err.Error())
					return
				}
				a.forwardEdit = ""
				a.startForwardFor(lr, key)
			}
		})
	})
}

func (a *App) openForward(local uint16) {
	open := a.openExternal
	if open == nil || local == 0 {
		return
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", local)
	go func() {
		if err := open(url); err != nil {
			a.setErr("open port forward: " + publicError(err))
		}
	}()
}

func (a *App) beginForwardEdit(key string, remote uint16, custom bool) {
	if custom {
		remote = 80
	}
	a.forwardLocal = strconv.Itoa(int(a.defaultLocalPort(remote)))
	a.forwardRemote = strconv.Itoa(int(remote))
	a.forwardEdit = key
}

func (a *App) defaultLocalPort(remote uint16) uint16 {
	a.mu.Lock()
	defer a.mu.Unlock()
	free := func(port uint16) bool {
		if port == 0 {
			return false
		}
		if _, starting := a.forwardStarting[port]; starting {
			return false
		}
		for _, fw := range a.forwards {
			if fw != nil && fw.Local == port && fw.Status() != "Stopped" {
				return false
			}
		}
		return true
	}
	if free(remote) {
		return remote
	}
	if parsed, err := strconv.ParseUint(strings.TrimSpace(a.forwardLocal), 10, 16); err == nil && free(uint16(parsed)) {
		return uint16(parsed)
	}
	for port := uint16(8080); port < 65535; port++ {
		if free(port) {
			return port
		}
	}
	return 65535
}

func (a *App) podForwardKey(lr listRow, remote uint16) string {
	return fmt.Sprintf("%s|%s|%s|%d", a.session.Current(), lr.Namespace, lr.Name, remote)
}

func (a *App) singlePodContainer(namespace, pod string) (string, bool) {
	a.mu.Lock()
	item := a.pods[namespace+"/"+pod]
	if item != nil {
		item = item.DeepCopy()
	}
	a.mu.Unlock()
	if item == nil || len(item.Spec.Containers) != 1 || item.Spec.Containers[0].Name == "" {
		return "", false
	}
	return item.Spec.Containers[0].Name, true
}

// readyDots summarizes ready application containers and excludes init containers.
func readyDots(dots []pods.Dot) string {
	ready, total := 0, 0
	for _, d := range dots {
		if d.Init {
			continue
		}
		total++
		if d.State == pods.DotReady {
			ready++
		}
	}
	return strconv.Itoa(ready) + "/" + strconv.Itoa(total)
}

// ensurePodUsageLocked reads pod metrics when a shown list's reading is
// due, and draws again once the next one is, so a hidden list reads
// nothing.
func (a *App) ensurePodUsageLocked(now time.Time, fallback []string) {
	if a.podUsageBusy || (!a.podUsageAt.IsZero() && now.Sub(a.podUsageAt) < podUsageEvery) {
		return
	}
	a.podUsageBusy = true
	gen := a.podUsageGen
	fetch := a.podUsageFetch
	if fetch == nil {
		fetch = a.session.PodUsages
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		usage, ok, err := fetch(ctx, fallback)
		cancel()
		a.mu.Lock()
		current := gen == a.podUsageGen
		if current {
			a.podUsageBusy = false
			a.podUsageAt = time.Now()
			// A failed read keeps the last one rather than turning every
			// row to N/A for a moment.
			if err == nil {
				if !ok {
					usage = nil
				}
				a.podUsage = usage
			}
		}
		a.mu.Unlock()
		if !current {
			return
		}
		a.invalidate()
		time.AfterFunc(podUsageEvery, a.invalidate)
	}()
}

// forgetPodUsageLocked drops the reading on a context switch. A read in
// flight lands nowhere.
func (a *App) forgetPodUsageLocked() {
	a.podUsage = nil
	a.podUsageAt = time.Time{}
	a.podUsageBusy = false
	a.podUsageGen++
}

// openListAt opens a list screen with the row at key chosen, as a pinned
// tab, so the list the link was in is not replaced. The search and filter
// are cleared so that row is not hidden.
func (a *App) openListAt(screen, key string) {
	a.showScreen(screen, false)
	a.listQuery = ""
	a.listFilter = 0
	a.listKey = key
	a.row = -1
	a.listReveal = true
	// The source tab's panel must not draw for a frame on this one.
	a.detailPanel = nil
	if screen == "workloads" {
		// The namespace filter is the tab's own; one that hides the row goes.
		if parts := strings.SplitN(key, "/", 3); len(parts) == 3 && a.nsFilter != "" && a.nsFilter != parts[1] {
			a.nsFilter = ""
		}
	}
	a.invalidate()
}

// podNamespaces lists the namespaces of items, sorted and capped.
func podNamespaces(items []*corev1.Pod) []string {
	seen := map[string]bool{}
	for _, p := range items {
		seen[p.Namespace] = true
	}
	out := make([]string, 0, len(seen))
	for ns := range seen {
		out = append(out, ns)
	}
	sort.Strings(out)
	if len(out) > podUsageSpread {
		out = out[:podUsageSpread]
	}
	return out
}
