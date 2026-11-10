package desktop

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/pods"
	"github.com/dacsang97/nakube/internal/views/resource"
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
		onEdit, onRemove := a.panelResourceActions(lr)
		a.offerDetailPanel(selection.Changed, true, shellDetailPanel{
			title:    "Pod: " + row.Name,
			label:    "Pod details panel",
			copyText: row.Name,
			onEdit:   onEdit,
			onRemove: onRemove,
			draw: func(c *ui.Context, t *ui.Theme) {
				a.drawPodMetrics(c, t, row.Namespace, row.Name)
				resource.Detail(c, t, a.podDetailList(row, lr), 0)
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

// podDetailList is the pod as a one-row generic list, so the panel keeps
// the same facts, port fields and Logs, Exec and Forward buttons.
func (a *App) podDetailList(row pods.Row, lr listRow) resource.List {
	owner := "—"
	if row.Owner.Kind != "" {
		owner = row.Owner.Kind + " " + row.Owner.Name
	}
	return resource.List{
		Columns: []string{"Namespace", "Name", "Status", "Ready", "Restarts", "Node", "Controlled By", "QoS"},
		Rows: []resource.Row{{Kind: "Pod", Name: row.Name, Cells: []string{
			row.Namespace, row.Name, row.Status, readyDots(row.Dots), strconv.Itoa(int(row.Restarts)), row.Node, owner, row.QoS,
		}}},
		ScaleTo:  &a.scaleTo,
		Local:    &a.forwardLocal,
		Remote:   &a.forwardRemote,
		Revision: &a.rollbackRev,
		OnAction: func(_ int, action string) { a.runAction(lr, action) },
	}
}

// readyDots counts ready app containers, as kubectl's READY column.
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
