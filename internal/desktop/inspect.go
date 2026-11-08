package desktop

import (
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
	"github.com/dacsang97/nakube/internal/views/events"
	"github.com/dacsang97/nakube/internal/views/namespaces"
	"github.com/dacsang97/nakube/internal/views/nodes"
	"github.com/dacsang97/nakube/internal/views/workloads"
)

func inspectScreen(id string) bool {
	switch id {
	case "nodes", "namespaces", "events":
		return true
	default:
		return false
	}
}

func (a *App) drawInspect(c *ui.Context, t *ui.Theme, id string) {
	switch id {
	case "nodes":
		a.drawNodes(c, t)
	case "namespaces":
		a.drawNamespaces(c, t)
	case "events":
		a.drawWithSelector(c, t, a.drawEvents)
	}
}

func (a *App) drawNodes(c *ui.Context, t *ui.Theme) {
	a.mu.Lock()
	items := nodeSlice(a.nodes)
	usage := a.usage
	loading := a.errText == "" && !a.kindReady("Node") && len(items) == 0
	a.mu.Unlock()
	a.prepareInspect("nodes")
	page := nodes.Build(items, nodeSamples(usage), a.listQuery, a.listFilter, a.listSort.Column, a.listSort.Descending, time.Now())
	keys := make([]string, len(page.Rows))
	for i, row := range page.Rows {
		keys[i] = row.Key
	}
	a.bindList(keys)
	selection := nodes.Draw(c, t, page, &a.listQuery, &a.listFilter, &a.listState, loading)
	if selection.Valid {
		row := page.Rows[selection.Index]
		resourceRow := listRow{Name: row.Name, Identity: row.Identity}
		onEdit, onRemove := a.panelResourceActions(resourceRow)
		a.offerDetailPanel(selection.Changed, true, shellDetailPanel{
			title:    row.Detail.Title,
			label:    "Node details panel",
			copyText: row.Detail.Title,
			onEdit:   onEdit,
			onRemove: onRemove,
			draw: func(c *ui.Context, t *ui.Theme) {
				kit.DetailBody(c, t, row.Detail, nil)
			},
		})
	} else {
		a.offerDetailPanel(selection.Changed, false, shellDetailPanel{})
	}
	a.finishInspect(keys)
}

func (a *App) drawNamespaces(c *ui.Context, t *ui.Theme) {
	a.mu.Lock()
	items := namespaceSlice(a.namespaces)
	loading := a.errText == "" && !a.kindReady("Namespace") && len(items) == 0
	a.mu.Unlock()
	a.prepareInspect("namespaces")
	page := namespaces.Build(items, a.listQuery, a.listFilter, a.listSort.Column, a.listSort.Descending, time.Now())
	keys := make([]string, len(page.Rows))
	for i, row := range page.Rows {
		keys[i] = row.Key
	}
	a.bindList(keys)
	selection := namespaces.Draw(c, t, page, &a.listQuery, &a.listFilter, &a.listState, loading)
	if selection.Valid {
		row := page.Rows[selection.Index]
		resourceRow := listRow{Name: row.Name, Identity: row.Identity}
		onEdit, onRemove := a.panelResourceActions(resourceRow)
		a.offerDetailPanel(selection.Changed, true, shellDetailPanel{
			title:    row.Detail.Title,
			label:    "Namespace details panel",
			copyText: row.Detail.Title,
			onEdit:   onEdit,
			onRemove: onRemove,
			draw: func(c *ui.Context, t *ui.Theme) {
				kit.DetailBody(c, t, row.Detail, nil)
			},
		})
	} else {
		a.offerDetailPanel(selection.Changed, false, shellDetailPanel{})
	}
	a.finishInspect(keys)
}

func (a *App) drawEvents(c *ui.Context, t *ui.Theme) {
	a.mu.Lock()
	items := eventSlice(a.events)
	loading := a.errText == "" && !a.kindReady("Event") && len(items) == 0
	a.mu.Unlock()
	a.prepareInspect("events")
	page := events.Build(items, a.listQuery, a.listSort.Column, a.listSort.Descending, time.Now(), a.nsFilter)
	keys := make([]string, len(page.Rows))
	for i, row := range page.Rows {
		keys[i] = row.Key
	}
	a.bindList(keys)
	selection := events.Draw(c, t, page, &a.listQuery, &a.listState, loading)
	if selection.Valid {
		row := page.Rows[selection.Index]
		resourceRow := listRow{Namespace: row.Namespace, Name: row.Identity.Name, Identity: row.Identity}
		onEdit, onRemove := a.panelResourceActions(resourceRow)
		a.offerDetailPanel(selection.Changed, true, shellDetailPanel{
			title:    row.Detail.Title,
			label:    "Event details panel",
			copyText: row.Detail.Title,
			onEdit:   onEdit,
			onRemove: onRemove,
			draw: func(c *ui.Context, t *ui.Theme) {
				kit.DetailBody(c, t, row.Detail, nil)
			},
		})
	} else {
		a.offerDetailPanel(selection.Changed, false, shellDetailPanel{})
	}
	a.finishInspect(keys)
}

func (a *App) prepareInspect(id string) {
	if a.listSort.Column == "" {
		switch id {
		case "events":
			a.listSort = ui.SortOrder{Column: events.ColAge, Descending: true}
		case "namespaces":
			a.listSort = ui.SortOrder{Column: namespaces.ColName}
		case "workloads":
			a.listSort = ui.SortOrder{Column: workloads.ColKind}
			a.row = -1
		default:
			a.listSort = ui.SortOrder{Column: nodes.ColName}
		}
	}
}

func (a *App) bindList(keys []string) {
	a.row = kit.Place(a.row, a.listKey, keys)
	a.listState.Selected = &a.row
	a.listState.Sort = &a.listSort
	a.listState.Columns = cloneLayout(a.listColumns)
	a.listState.KeepOffset = true
	a.listState.RoundRows = true
	a.listState.Key = func(i int) any {
		if i < 0 || i >= len(keys) {
			return i
		}
		return keys[i]
	}
}

func (a *App) finishInspect(keys []string) {
	a.listColumns = cloneLayout(a.listState.Columns)
	if a.row >= 0 && a.row < len(keys) {
		a.listKey = keys[a.row]
		return
	}
	a.listKey = ""
}

func cloneLayout(layout ui.TableLayout) ui.TableLayout {
	out := ui.TableLayout{Order: append([]string(nil), layout.Order...)}
	if len(layout.Widths) == 0 {
		return out
	}
	out.Widths = make(map[string]float32, len(layout.Widths))
	for id, width := range layout.Widths {
		out.Widths[id] = width
	}
	return out
}

func nodeSlice(items map[string]*corev1.Node) []*corev1.Node {
	out := make([]*corev1.Node, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

func namespaceSlice(items map[string]*corev1.Namespace) []*corev1.Namespace {
	out := make([]*corev1.Namespace, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

func eventSlice(items map[string]*corev1.Event) []*corev1.Event {
	out := make([]*corev1.Event, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

func nodeSamples(usage cluster.Usage) map[string]nodes.Sample {
	if !usage.Available {
		return nil
	}
	out := make(map[string]nodes.Sample, len(usage.Nodes))
	for _, item := range usage.Nodes {
		out[item.Name] = nodes.Sample{CPUMilli: item.CPUMilli, MemoryBytes: item.MemoryBytes}
	}
	return out
}
