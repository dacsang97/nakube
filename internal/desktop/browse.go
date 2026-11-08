package desktop

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/resource"
	"github.com/dacsang97/nakube/internal/views/workloads"
)

// listRow is one table row. Actions use Identity, captured when the row was built.
type listRow struct {
	Cells     []string
	Namespace string
	Name      string
	Identity  cluster.Identity
	Forward   *cluster.Forward
}

func (a *App) panelResourceActions(row listRow) (func(), func()) {
	editAction, removeAction := resource.HeaderActions(row.Identity.Kind)
	action := func(name string) func() {
		if name == "" {
			return nil
		}
		return func() {
			a.runAction(row, name)
		}
	}
	return action(editAction), action(removeAction)
}

type listSnap struct {
	columns []string
	rows    []listRow
	loading bool
	err     string
	notice  string
}

func (a *App) drawMain(c *ui.Context, t *ui.Theme, id string, snap listSnap) {
	if a.mode == "logs" {
		a.drawLogs(c, t)
		return
	}
	if a.mode == "exec" {
		a.drawExec(c, t)
		return
	}
	if a.mode == "yaml" {
		a.drawYAML(c, t)
		return
	}
	if id == "workloads" {
		a.drawWithSelector(c, t, a.drawWorkloads)
		return
	}
	if id == "overview" || id == "" {
		return
	}
	if isArgoScreen(id) {
		a.drawArgo(c, t, id)
		return
	}
	if id == "helm" {
		a.drawList(c, t, id, snap)
		a.drawHelm(c, t)
		return
	}
	a.drawWithSelector(c, t, func(c *ui.Context, t *ui.Theme) {
		a.drawList(c, t, id, snap)
	})
}

// drawWithSelector stacks the namespace chip row above a list screen. Only
// namespace-scoped screens call it.
func (a *App) drawWithSelector(c *ui.Context, t *ui.Theme, draw func(*ui.Context, *ui.Theme)) {
	ui.Column(c).Gap(8).FillWidth().FillHeight().Children(func() {
		a.drawNamespaceSelector(c, t)
		draw(c, t)
	})
}

func (a *App) drawList(c *ui.Context, t *ui.Theme, selected string, snap listSnap) {
	cols := snap.columns
	if snap.loading && len(cols) == 0 {
		cols = placeholderColumns(selected)
	}
	rows := make([]resource.Row, len(snap.rows))
	for i, row := range snap.rows {
		rows[i] = resource.Row{Kind: row.Identity.Kind, Name: row.Name, Cells: row.Cells}
	}
	list := resource.List{
		Title:    labelFor(selected),
		Err:      snap.err,
		Notice:   snap.notice,
		Empty:    emptyText(selected),
		Loading:  snap.loading,
		Columns:  cols,
		Rows:     rows,
		Selected: &a.row,
		ScaleTo:  &a.scaleTo,
		Local:    &a.forwardLocal,
		Remote:   &a.forwardRemote,
		Revision: &a.rollbackRev,
		OnAction: func(index int, action string) {
			if index < 0 || index >= len(snap.rows) {
				return
			}
			a.runAction(snap.rows[index], action)
		},
	}
	selection := resource.Draw(c, t, list)
	if selection.Valid {
		row := snap.rows[selection.Index]
		onEdit, onRemove := a.panelResourceActions(row)
		a.offerDetailPanel(selection.Changed, true, shellDetailPanel{
			title:    row.Identity.Kind + ": " + row.Name,
			label:    row.Identity.Kind + " details panel",
			copyText: row.Name,
			onEdit:   onEdit,
			onRemove: onRemove,
			draw: func(c *ui.Context, t *ui.Theme) {
				resource.Detail(c, t, list, selection.Index)
			},
		})
	} else {
		a.offerDetailPanel(selection.Changed, false, shellDetailPanel{})
	}
}

func emptyText(id string) string {
	switch id {
	case "port-forwards":
		return "None in this process. Start one from a Pod or Service."
	case "helm":
		return "No releases"
	default:
		return "None"
	}
}

func watchKind(id string) string {
	switch id {
	case "pods", "workloads":
		return "Pod"
	case "nodes":
		return "Node"
	case "namespaces":
		return "Namespace"
	case "events":
		return "Event"
	default:
		return ""
	}
}

func placeholderColumns(id string) []string {
	switch id {
	case "helm":
		return []string{"Namespace", "Name", "Chart", "Revision", "Status"}
	case "argo-applications":
		return []string{"Namespace", "Name", "Sync", "Health"}
	case "pods":
		return []string{"Namespace", "Name", "Phase", "Ready"}
	case "workloads":
		return []string{"Phase", "Pods"}
	case "port-forwards":
		return []string{"Namespace", "Pod", "Local", "Remote"}
	default:
		return []string{"Namespace", "Name", "Age"}
	}
}

func (a *App) runAction(row listRow, action string) {
	switch action {
	case "logs":
		a.openLogs(row.Namespace, row.Name, "")
	case "exec":
		a.openExec(row.Namespace, row.Name, "")
	case "yaml":
		a.openYAML(row.Identity)
	case "forward":
		a.startForward(row)
	case "stop":
		a.stopForward(row.Forward)
	default:
		a.ask(row, action)
	}
}

func (a *App) ask(row listRow, action string) {
	c := Confirm{
		Open:      true,
		Kind:      row.Identity.Kind,
		Namespace: row.Namespace,
		Name:      row.Name,
		Action:    action,
		GVR:       row.Identity.GVR,
	}
	if action == "scale" {
		n, err := strconv.Atoi(strings.TrimSpace(a.scaleTo))
		if err != nil || n < 0 {
			a.setErr("replicas must be a number")
			return
		}
		c.Replicas = int32(n)
	}
	if action == "rollback" {
		n, err := strconv.ParseInt(strings.TrimSpace(a.rollbackRev), 10, 64)
		if err != nil || n < 0 {
			a.setErr("revision must be a number")
			return
		}
		c.Revision = n
	}
	a.mu.Lock()
	a.confirm = &c
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) applyConfirm() {
	a.mu.Lock()
	if a.confirm == nil || !a.confirm.Open {
		a.mu.Unlock()
		return
	}
	c := *a.confirm
	a.confirm.Open = false
	selected := a.selected
	a.mu.Unlock()
	go func() {
		err := a.runConfirmed(c)
		if err != nil {
			a.setErr(publicError(err))
			return
		}
		a.setErr("")
		a.loadList(selected)
	}()
}

func (a *App) runConfirmed(c Confirm) error {
	ctx := context.Background()
	id := cluster.Identity{GVR: c.GVR, Kind: c.Kind, Namespace: c.Namespace, Name: c.Name}
	switch c.Action {
	case "delete":
		if c.Kind == "Application" {
			return a.session.DeleteArgo(ctx, c.Namespace, c.Name, c.Cascade)
		}
		return a.session.Delete(ctx, id)
	case "restart":
		return a.session.Restart(ctx, id)
	case "scale":
		return a.session.Scale(ctx, id, c.Replicas)
	case "sync":
		return a.session.SyncArgo(ctx, c.Namespace, c.Name, cluster.ArgoSyncOptions{Revision: c.SyncRevision, Prune: c.Prune})
	case "sync-drift":
		return applySyncTargets(ctx, a, c)
	case "refresh":
		return a.session.RefreshArgo(ctx, c.Namespace, c.Name, "normal")
	case "refresh-all":
		return applyRefreshTargets(ctx, a, c)
	case "hard-refresh":
		return a.session.RefreshArgo(ctx, c.Namespace, c.Name, "hard")
	case "rollback":
		if c.Kind == "Release" {
			return a.session.HelmRollback(ctx, c.Namespace, c.Name, int(c.Revision))
		}
		return a.session.RollbackArgo(ctx, c.Namespace, c.Name, c.Revision, false)
	case "uninstall":
		return a.session.HelmUninstall(ctx, c.Namespace, c.Name)
	default:
		return fmt.Errorf("unknown action %s", c.Action)
	}
}

func (a *App) startForward(row listRow) {
	local, remote, err := a.forwardPorts()
	if err != nil {
		a.setErr(err.Error())
		return
	}
	go func() {
		pod := row.Name
		if row.Identity.Kind == "Service" {
			pod, err = a.session.ReadyPod(context.Background(), row.Namespace, row.Name)
			if err != nil {
				a.setErr(publicError(err))
				return
			}
		}
		fw, err := a.session.PortForward(context.Background(), row.Namespace, pod, local, remote)
		if err != nil {
			a.setErr(publicError(err))
			return
		}
		a.mu.Lock()
		a.forwards = append(a.forwards, fw)
		a.mu.Unlock()
		a.setErr("")
		a.invalidate()
	}()
}

func (a *App) forwardPorts() (uint16, uint16, error) {
	local, err := strconv.ParseUint(strings.TrimSpace(a.forwardLocal), 10, 16)
	if err != nil || local == 0 {
		return 0, 0, fmt.Errorf("local port must be a number")
	}
	remote, err := strconv.ParseUint(strings.TrimSpace(a.forwardRemote), 10, 16)
	if err != nil || remote == 0 {
		return 0, 0, fmt.Errorf("remote port must be a number")
	}
	return uint16(local), uint16(remote), nil
}

func (a *App) stopForward(fw *cluster.Forward) {
	if fw != nil {
		fw.Stop()
	}
	a.mu.Lock()
	next := make([]*cluster.Forward, 0, len(a.forwards))
	for _, item := range a.forwards {
		if item != fw {
			next = append(next, item)
		}
	}
	a.forwards = next
	a.mu.Unlock()
	a.invalidate()
}

func (a *App) leaveOverlay() {
	if a.logCancel != nil {
		a.logCancel()
		a.logCancel = nil
	}
	a.logLoading = false
	if a.mode == "exec" {
		a.closeExec()
	}
	a.mode = ""
	a.yaml = nil
	a.stash()
}

func (a *App) backFromOverlay() {
	if a.tabs != nil && a.tabs.Detail(a.active) != nil {
		a.closeTab(a.active)
		return
	}
	a.leaveOverlay()
}

func (a *App) backFromExec() {
	if a.tabs != nil && a.tabs.Detail(a.active) != nil {
		a.closeTab(a.active)
		return
	}
	a.closeExec()
}

func (a *App) backFromYAML() {
	if a.tabs != nil && a.tabs.Detail(a.active) != nil {
		a.closeTab(a.active)
		return
	}
	a.mode = ""
	a.yaml = nil
	a.stash()
}

func (a *App) loadList(id string) {
	if isLive(id) {
		return
	}
	a.ensureTabs()
	tabID := a.active
	a.mu.Lock()
	b := a.bodies[tabID]
	if b == nil {
		b = newTabBody()
		if a.bodies == nil {
			a.bodies = map[string]*tabBody{}
		}
		a.bodies[tabID] = b
	}
	b.listGen++
	gen := b.listGen
	b.listID = id
	b.loaded = false
	b.clearList()
	if a.active == tabID {
		a.projectListLocked(b)
	}
	a.mu.Unlock()
	a.invalidate()
	if isArgoScreen(id) {
		go a.loadArgo(tabID, gen, id)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cols, rows, notice, err := a.fetchList(ctx, id)
		a.mu.Lock()
		defer a.mu.Unlock()
		b := a.bodies[tabID]
		if b == nil || gen != b.listGen {
			return
		}
		b.listLoading = false
		b.loaded = true
		b.listCols = cols
		b.listRows = rows
		b.listNotice = notice
		b.listErr = ""
		if err != nil {
			b.listErr = publicError(err)
		}
		if a.active == tabID {
			a.projectListLocked(b)
		}
		a.invalidate()
	}()
}

func isLive(id string) bool {
	switch id {
	case "", "overview", "workloads", "pods", "nodes", "namespaces", "events", "port-forwards", "settings", "catalog":
		return true
	default:
		return false
	}
}

func (a *App) snapshotLocked(id string) listSnap {
	if isLive(id) {
		cols, rows := a.liveRowsLocked(id)
		return listSnap{columns: cols, rows: rows}
	}
	if a.listID != id {
		return listSnap{loading: true}
	}
	rows := append([]listRow(nil), a.listRows...)
	return listSnap{columns: append([]string(nil), a.listCols...), rows: rows, loading: a.listLoading, err: a.listErr, notice: a.listNotice}
}

func (a *App) liveRowsLocked(id string) ([]string, []listRow) {
	switch id {
	case "workloads":
		return nil, nil
	case "pods":
		return podTable(a.pods)
	case "port-forwards":
		return forwardTable(a.forwards)
	default:
		return nil, nil
	}
}

func (a *App) fetchList(ctx context.Context, id string) ([]string, []listRow, string, error) {
	switch {
	case id == "helm":
		rels, err := a.session.HelmList(ctx)
		if err != nil {
			return nil, nil, "", err
		}
		return helmRows(rels)
	case strings.HasPrefix(id, "istio-"):
		return a.istioRows(ctx, id)
	case workloadKind(id).Kind != "":
		return a.genericRows(ctx, workloadKind(id), "")
	default:
		kind, ok := LookupKind(id)
		if !ok {
			return nil, nil, "", fmt.Errorf("unknown screen %s", id)
		}
		notice := ""
		rows, err := a.session.ListGeneric(ctx, kind.GVR, "")
		if err != nil {
			if apierrors.IsNotFound(err) {
				return []string{"Namespace", "Name", "Age"}, nil, "CRD is not installed", nil
			}
			return nil, nil, "", err
		}
		return genericColumns(), a.filterListRows(genericListRows(rows, kind.GVR, kindLabel(kind.ID))), notice, nil
	}
}

func (a *App) setNamespaceFilter(ns string) {
	a.nsFilter = ns
	a.row = -1
	a.nsPaletteOpen = false
	a.nsPaletteQuery = ""
	a.invalidate()
}

// namespacePaletteItems lists namespaces sorted by name, filtered by the
// palette query. An empty query returns all.
func (a *App) namespacePaletteItems() []string {
	names := make([]string, 0, len(a.namespaces))
	for name := range a.namespaces {
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	q := strings.ToLower(strings.TrimSpace(a.nsPaletteQuery))
	if q == "" {
		return names
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		if strings.Contains(strings.ToLower(name), q) {
			out = append(out, name)
		}
	}
	return out
}

// drawNamespacePalette renders the Cmd/Ctrl+P overlay: a search field plus
// the filtered namespace list. Enter applies the first match; Esc or a
// backdrop click closes, handled by the modal.
func (a *App) drawNamespacePalette(c *ui.Context, t *ui.Theme) {
	if !a.nsPaletteOpen {
		return
	}
	ui.Modal(c, &a.nsPaletteOpen, func() {
		c.OnShortcut(0, ui.KeyEnter, func() {
			if items := a.namespacePaletteItems(); len(items) > 0 {
				a.setNamespaceFilter(items[0])
			}
		})
		ui.Column(c).Width(320).Padding(12).Gap(8).Children(func() {
			ui.Row(c).Children(func() {
				ui.TextInput(c, &a.nsPaletteQuery).Placeholder("Namespace").Grow(1)
			})
			ui.Scroll(c).MaxHeight(280).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for _, name := range a.namespacePaletteItems() {
						n := name
						row := ui.Row(c).Height(28).Padding(0, 8).Radius(4).AlignItems(ui.Center)
						if n == a.nsFilter {
							row.Background(t.SurfacePressed)
						}
						ui.Text(c, n).SingleLine()
						if row.Clicked() {
							a.setNamespaceFilter(n)
						}
					}
					if len(a.namespacePaletteItems()) == 0 {
						ui.Text(c, "No matches").TextColor(t.TextMuted).Padding(8)
					}
				})
			})
		})
	})
}

// drawNamespaceSelector renders the All + per-namespace chip row above list
// screens. Namespaces come from the informer cache, sorted by name.
func (a *App) drawNamespaceSelector(c *ui.Context, t *ui.Theme) {
	names := make([]string, 0, len(a.namespaces))
	for name := range a.namespaces {
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) <= 1 {
		return
	}
	ui.Row(c).Gap(4).Wrap().Padding(0, 0, 8, 0).Children(func() {
		all := a.chip(c, t, "All", a.nsFilter == "")
		if all.Clicked() {
			a.setNamespaceFilter("")
		}
		for _, name := range names {
			on := name == a.nsFilter
			btn := a.chip(c, t, name, on)
			if btn.Clicked() {
				a.setNamespaceFilter(name)
			}
		}
	})
}

func (a *App) chip(c *ui.Context, t *ui.Theme, label string, on bool) ui.Element {
	btn := ui.Button(c, label).FontSize(11).Padding(1, 8).Radius(8)
	if on {
		btn.Background(t.AccentText).TextColor(t.Accent)
	} else {
		btn.Background(t.Surface).TextColor(t.TextMuted)
	}
	return btn
}

func (a *App) filterListRows(rows []listRow) []listRow {
	if a.nsFilter == "" {
		return rows
	}
	out := make([]listRow, 0, len(rows))
	for _, row := range rows {
		if row.Namespace == a.nsFilter {
			out = append(out, row)
		}
	}
	return out
}

func (a *App) genericRows(ctx context.Context, kind namedKind, missing string) ([]string, []listRow, string, error) {
	rows, err := a.session.ListGeneric(ctx, kind.GVR, "")
	if err != nil {
		if missing != "" && apierrors.IsNotFound(err) {
			return genericColumns(), nil, missing, nil
		}
		return nil, nil, "", err
	}
	return genericColumns(), a.filterListRows(genericListRows(rows, kind.GVR, kind.Kind)), "", nil
}

func (a *App) istioRows(ctx context.Context, id string) ([]string, []listRow, string, error) {
	var group string
	for _, kind := range cluster.IstioKinds {
		if kind.ID == id {
			group = kind.Group
		}
	}
	if group == "" {
		return nil, nil, "", fmt.Errorf("unknown istio screen %s", id)
	}
	resource := strings.TrimPrefix(id, "istio-")
	var last error
	for _, ver := range []string{"v1", "v1beta1"} {
		gvr := schema.GroupVersionResource{Group: group, Version: ver, Resource: resource}
		rows, err := a.session.ListGeneric(ctx, gvr, "")
		if err == nil {
			return genericColumns(), a.filterListRows(genericListRows(rows, gvr, istioKindName(id))), "", nil
		}
		last = err
		if !apierrors.IsNotFound(err) {
			return nil, nil, "", err
		}
	}
	if apierrors.IsNotFound(last) {
		return genericColumns(), nil, "CRD is not installed", nil
	}
	return nil, nil, "", last
}

type namedKind struct {
	Kind string
	GVR  schema.GroupVersionResource
}

func workloadKind(id string) namedKind {
	switch id {
	case "pods":
		return namedKind{"Pod", schema.GroupVersionResource{Version: "v1", Resource: "pods"}}
	case "deployments":
		return namedKind{"Deployment", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}}
	case "replicasets":
		return namedKind{"ReplicaSet", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}}
	case "daemonsets":
		return namedKind{"DaemonSet", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}}
	case "statefulsets":
		return namedKind{"StatefulSet", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}}
	case "jobs":
		return namedKind{"Job", schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}}
	case "cronjobs":
		return namedKind{"CronJob", schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}}
	default:
		return namedKind{}
	}
}

func genericColumns() []string {
	return []string{"Namespace", "Name", "Age"}
}

func genericListRows(rows []cluster.GenericRow, gvr schema.GroupVersionResource, kind string) []listRow {
	out := make([]listRow, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		out = append(out, listRow{
			Cells:     []string{row.Namespace, row.Name, ageText(row.Created, now)},
			Namespace: row.Namespace,
			Name:      row.Name,
			Identity:  cluster.Identity{GVR: gvr, Kind: kind, Namespace: row.Namespace, Name: row.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func helmRows(rels []cluster.HelmRelease) ([]string, []listRow, string, error) {
	cols := []string{"Namespace", "Name", "Chart", "Revision", "Status"}
	out := make([]listRow, 0, len(rels))
	for _, rel := range rels {
		out = append(out, listRow{
			Cells:     []string{rel.Namespace, rel.Name, rel.Chart, strconv.Itoa(rel.Revision), rel.Status},
			Namespace: rel.Namespace,
			Name:      rel.Name,
			Identity:  cluster.Identity{Kind: "Release", Namespace: rel.Namespace, Name: rel.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return cols, out, "", nil
}

func (a *App) drawWorkloads(c *ui.Context, t *ui.Theme) {
	a.mu.Lock()
	objects := workloadSlice(a.workloadObjects)
	loading := a.errText == "" && !a.workloadsReadyLocked() && len(objects) == 0
	contextName := a.current
	a.mu.Unlock()

	a.prepareInspect("workloads")
	page := workloads.Build(objects, a.listQuery, a.listFilter, a.listSort.Column, a.listSort.Descending, a.nsFilter)
	keys := make([]string, len(page.Rows))
	for i, row := range page.Rows {
		keys[i] = row.Key
	}
	a.bindList(keys)
	selection := workloads.Draw(c, t, page, contextName, &a.listQuery, &a.listFilter, &a.listState, loading, time.Now(), a.openPage)
	item := selection.Row
	kind := workloadKind(item.KindID)
	row := listRow{
		Namespace: item.Namespace,
		Name:      item.Name,
		Identity: cluster.Identity{
			GVR:       kind.GVR,
			Kind:      item.Kind,
			Namespace: item.Namespace,
			Name:      item.Name,
		},
	}
	onEdit, onRemove := a.panelResourceActions(row)
	a.offerDetailPanel(selection.Changed, selection.Valid, shellDetailPanel{
		title:    item.Kind + ": " + item.Name,
		label:    "Workload details panel",
		copyText: item.Name,
		onEdit:   onEdit,
		onRemove: onRemove,
		draw: func(c *ui.Context, t *ui.Theme) {
			workloads.Detail(c, t, item, time.Now())
		},
	})
	a.finishInspect(keys)
}

func (a *App) workloadsReadyLocked() bool {
	for _, kind := range []string{"Pod", "Deployment", "ReplicaSet", "DaemonSet", "StatefulSet", "Job", "CronJob"} {
		if !a.kindReady(kind) {
			return false
		}
	}
	return true
}

func workloadSlice(items map[string]runtime.Object) []runtime.Object {
	out := make([]runtime.Object, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

func podTable(pods map[string]*corev1.Pod) ([]string, []listRow) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	out := make([]listRow, 0, len(pods))
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		out = append(out, listRow{
			Cells:     []string{pod.Namespace, pod.Name, string(pod.Status.Phase), readyCount(pod)},
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Identity:  cluster.Identity{GVR: gvr, Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return []string{"Namespace", "Name", "Phase", "Ready"}, out
}

func forwardTable(items []*cluster.Forward) ([]string, []listRow) {
	out := make([]listRow, 0, len(items))
	for _, fw := range items {
		if fw == nil {
			continue
		}
		out = append(out, listRow{
			Cells:     []string{fw.Namespace, fw.Pod, strconv.Itoa(int(fw.Local)), strconv.Itoa(int(fw.Remote))},
			Namespace: fw.Namespace,
			Name:      fw.Pod,
			Identity:  cluster.Identity{Kind: "Forward", Namespace: fw.Namespace, Name: fw.Pod},
			Forward:   fw,
		})
	}
	return []string{"Namespace", "Pod", "Local", "Remote"}, out
}

func readyCount(pod *corev1.Pod) string {
	var ready, total int
	for _, cs := range pod.Status.ContainerStatuses {
		total++
		if cs.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func ageText(created, now time.Time) string {
	if created.IsZero() {
		return ""
	}
	d := now.Sub(created)
	if d < 0 {
		d = 0
	}
	return d.Truncate(time.Second).String()
}

func kindLabel(id string) string {
	switch id {
	case "nodes":
		return "Node"
	case "namespaces":
		return "Namespace"
	case "events":
		return "Event"
	case "leases":
		return "Lease"
	case "services":
		return "Service"
	case "ingresses":
		return "Ingress"
	case "endpointslices":
		return "EndpointSlice"
	case "networkpolicies":
		return "NetworkPolicy"
	case "ingressclasses":
		return "IngressClass"
	case "configmaps":
		return "ConfigMap"
	case "secrets":
		return "Secret"
	case "horizontalpodautoscalers":
		return "HorizontalPodAutoscaler"
	case "limitranges":
		return "LimitRange"
	case "resourcequotas":
		return "ResourceQuota"
	case "poddisruptionbudgets":
		return "PodDisruptionBudget"
	case "persistentvolumes":
		return "PersistentVolume"
	case "persistentvolumeclaims":
		return "PersistentVolumeClaim"
	case "volumeattachments":
		return "VolumeAttachment"
	case "storageclasses":
		return "StorageClass"
	case "csidrivers":
		return "CSIDriver"
	case "csinodes":
		return "CSINode"
	case "serviceaccounts":
		return "ServiceAccount"
	case "roles":
		return "Role"
	case "rolebindings":
		return "RoleBinding"
	case "clusterroles":
		return "ClusterRole"
	case "clusterrolebindings":
		return "ClusterRoleBinding"
	case "crds":
		return "CustomResourceDefinition"
	case "priorityclasses":
		return "PriorityClass"
	case "runtimeclasses":
		return "RuntimeClass"
	default:
		return id
	}
}

func istioKindName(id string) string {
	for _, kind := range cluster.IstioKinds {
		if kind.ID == id {
			return kind.Kind
		}
	}
	return id
}
