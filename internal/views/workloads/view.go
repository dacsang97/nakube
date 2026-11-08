package workloads

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

var filters = []string{"All", "Healthy", "Active", "Attention", "Suspended"}

type Selection struct {
	Row     Row
	Valid   bool
	Changed bool
}

func Draw(c *ui.Context, t *ui.Theme, page Page, contextName string, query *string, filter *int, state *ui.ListState, loading bool, now time.Time, onOpen func(string), headerControl func(*ui.Context, *ui.Theme)) Selection {
	if now.IsZero() {
		now = time.Now()
	}
	changed := false
	ui.Box(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
		ui.Scroll(c).Fill().Scrollbars(false).Children(func() {
			ui.Column(c).Gap(14).Children(func() {
				header(c, t, page, contextName, loading, headerControl)
				summary(c, t, page, loading)
				visuals(c, t, page, loading, onOpen)
				changed = inventory(c, t, page, query, filter, state, loading, now)
			})
		})
	})
	row, valid := selected(page.Rows, state)
	return Selection{Row: row, Valid: valid, Changed: changed}
}

func header(c *ui.Context, t *ui.Theme, page Page, contextName string, loading bool, headerControl func(*ui.Context, *ui.Theme)) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Column(c).Grow(1).MinWidth(220).Gap(2).Children(func() {
			kit.PageHeader(c, t, "Workloads", "", headerControl)
			line := contextName
			if line == "" {
				line = "No context"
			}
			if !loading {
				line += "  ·  " + fmt.Sprintf("%d resources across %d kinds", page.Total, nonEmptyKinds(page.Kinds))
			}
			ui.Text(c, line).TextColor(t.TextMuted).SingleLine()
		})
		ui.Text(c, "Live inventory").FontSize(12).TextColor(t.TextMuted)
	})
}

func summary(c *ui.Context, t *ui.Theme, page Page, loading bool) {
	ui.Row(c).Gap(10).Wrap().AlignItems(ui.Stretch).Children(func() {
		metric(c, t, "Total", page.Total, t.Accent, loading)
		metric(c, t, "Healthy", page.Healthy, t.Success, loading)
		metric(c, t, "Active", page.Active, t.Warning, loading)
		metric(c, t, "Attention", page.Attention, t.Danger, loading)
	})
}

func metric(c *ui.Context, t *ui.Theme, label string, value int, color ui.Color, loading bool) {
	kit.Panel(c, t).Grow(1).Basis(0).MinWidth(150).Gap(6).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(8, 8).Radius(4).Background(color).Shrink(0)
			ui.Text(c, label).TextColor(t.TextMuted).FontSize(12)
		})
		if loading {
			kit.Bar(c, t, 48, 28)
			return
		}
		ui.Text(c, fmt.Sprintf("%d", value)).FontSize(28).Bold()
	})
}

func visuals(c *ui.Context, t *ui.Theme, page Page, loading bool, onOpen func(string)) {
	ui.Row(c).Gap(12).Wrap().AlignItems(ui.Stretch).Children(func() {
		readiness(c, t, page, loading)
		kindHealth(c, t, page, loading, onOpen)
	})
}

func readiness(c *ui.Context, t *ui.Theme, page Page, loading bool) {
	kit.Panel(c, t).Grow(1).Basis(0).MinWidth(280).Children(func() {
		ui.Text(c, "Overall readiness").FontSize(16).Bold()
		ui.Text(c, "Ready replicas against desired capacity").FontSize(12).TextColor(t.TextMuted)
		if loading {
			ui.Row(c).Gap(18).AlignItems(ui.Center).Children(func() {
				kit.Circle(c, t, 116)
				ui.Column(c).Grow(1).Gap(10).Children(func() {
					for range 4 {
						kit.Bar(c, t, 0, 12)
					}
				})
			})
			return
		}
		ui.Row(c).Gap(18).AlignItems(ui.Center).Children(func() {
			center := "—"
			if page.Desired > 0 {
				center = fmt.Sprintf("%.0f%%", 100*float64(page.Ready)/float64(page.Desired))
			}
			kit.Donut(c, t, 116, center, "ready", []kit.Ring{
				{Value: float64(page.Ready), Total: float64(page.Desired), Color: t.Success, Known: page.Desired > 0},
				{Value: float64(page.Healthy), Total: float64(page.Total), Color: t.Accent, Known: page.Total > 0},
			})
			kit.Legend(c, t, []kit.Swatch{
				{Color: t.Success, Name: "Ready", Value: fmt.Sprintf("%d/%d", page.Ready, page.Desired)},
				{Color: t.Accent, Name: "Healthy", Value: fmt.Sprintf("%d/%d", page.Healthy, page.Total)},
				{Color: t.Warning, Name: "Active", Value: fmt.Sprintf("%d", page.Active)},
				{Color: t.Danger, Name: "Attention", Value: fmt.Sprintf("%d", page.Attention)},
				{Color: t.Warning, Name: "Suspended", Value: fmt.Sprintf("%d", page.Suspended)},
			})
		})
	})
}

func kindHealth(c *ui.Context, t *ui.Theme, page Page, loading bool, onOpen func(string)) {
	kit.Panel(c, t).Grow(1.4).Basis(0).MinWidth(360).Children(func() {
		ui.Text(c, "Health by kind").FontSize(16).Bold()
		ui.Text(c, "Each rail is proportional to the resources in that kind").FontSize(12).TextColor(t.TextMuted)
		for _, kind := range page.Kinds {
			kind := kind
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				button := ui.Button(c, kind.Title).Width(112).Justify(ui.Start).Padding(0).Label("Open " + kind.Title)
				button.Border(0, ui.Transparent).Background(ui.Transparent)
				if button.Clicked() && onOpen != nil {
					onOpen(kind.ID)
				}
				if loading {
					kit.Bar(c, t, 0, 10)
					return
				}
				statusRail(c, t, kind)
				ui.Text(c, fmt.Sprintf("%d", kind.Total)).Width(28).TextAlign(ui.End).FontSize(12).TextColor(t.TextMuted)
			})
		}
	})
}

func statusRail(c *ui.Context, t *ui.Theme, kind KindSummary) {
	track := ui.Row(c).Grow(1).MinWidth(0).Height(10).Radius(5).Clip().Background(kit.Track(t))
	track.Label(kind.Title + " status distribution")
	track.Children(func() {
		if kind.Total == 0 {
			return
		}
		for _, status := range StatusOrder() {
			count := kind.Count(status)
			if count == 0 {
				continue
			}
			ui.Box(c).Height(10).Basis(0).Grow(float32(count)).Background(statusColor(t, status)).Tooltip(fmt.Sprintf("%s: %d", status.Label(), count))
		}
	})
}

func inventory(c *ui.Context, t *ui.Theme, page Page, query *string, filter *int, state *ui.ListState, loading bool, now time.Time) bool {
	changed := false
	kit.Panel(c, t).Padding(0).Gap(0).Children(func() {
		ui.Row(c).Padding(16, 16, 10, 16).Gap(8).AlignItems(ui.Baseline).Children(func() {
			ui.Text(c, "Inventory").FontSize(16).Bold()
			if !loading {
				ui.Text(c, CountLabel(len(page.Rows), page.Total)).TextColor(t.TextMuted).FontSize(12)
			}
		})
		ui.Row(c).Padding(0, 16, 12, 16).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			if query != nil {
				ui.SearchField(c, query).Grow(1).MinWidth(180).Label("Search workloads")
			}
			if filter != nil {
				ui.Segmented(c, filter, filters...).Label("Workload health")
			}
		})
		switch {
		case loading && page.Total == 0:
			ui.Column(c).Padding(0, 16, 16, 16).Gap(10).Children(func() {
				for range 6 {
					kit.Bar(c, t, 0, 14)
				}
			})
		case len(page.Rows) == 0:
			message := "No workloads found in this cluster."
			if page.Total > 0 {
				message = "No workloads match the current search and health filter."
			}
			ui.Text(c, message).Padding(4, 16, 20, 16).TextColor(t.TextMuted)
		default:
			changed = ui.Table(c, state, columns(), len(page.Rows), func(row, col int) {
				drawCell(c, t, page.Rows[row], row, col, state, now)
			}).Height(360).FillWidth().Changed()
		}
	})
	return changed
}

func columns() []ui.TableColumn {
	return []ui.TableColumn{
		{ID: ColKind, Title: "Kind", Width: 124, MinWidth: 96, Sortable: true},
		{ID: ColNamespace, Title: "Namespace", Width: 150, MinWidth: 110, Sortable: true},
		{ID: ColName, Title: "Name", MinWidth: 180, Sortable: true},
		{ID: ColStatus, Title: "Status", Width: 116, MinWidth: 100, Sortable: true},
		{ID: ColReady, Title: "Ready", Width: 76, MinWidth: 68, Align: ui.End, Sortable: true},
		{ID: ColAge, Title: "Age", Width: 76, MinWidth: 68, Align: ui.End, Sortable: true},
	}
}

func drawCell(c *ui.Context, t *ui.Theme, item Row, row, col int, state *ui.ListState, now time.Time) {
	on := kit.Chosen(state, row, col)
	switch col {
	case 0:
		kit.Line(c, t, item.Kind, kit.ToneNone, on)
	case 1:
		kit.Line(c, t, item.Namespace, kit.ToneMuted, on)
	case 2:
		kit.Line(c, t, item.Name, kit.ToneNone, on)
	case 3:
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			color := statusColor(t, item.Status)
			if on {
				color = t.AccentText
			}
			ui.Box(c).Size(7, 7).Radius(4).Background(color).Shrink(0)
			kit.Line(c, t, item.Status.Label(), statusTone(item.Status), on)
		})
	case 4:
		kit.Line(c, t, item.ReadyText(), kit.ToneNone, on)
	case 5:
		kit.Line(c, t, kit.Age(item.Created, now), kit.ToneMuted, on)
	}
}

func selected(rows []Row, state *ui.ListState) (Row, bool) {
	if state == nil || state.Selected == nil {
		return Row{}, false
	}
	index := *state.Selected
	if index < 0 || index >= len(rows) {
		return Row{}, false
	}
	return rows[index], true
}

// Detail draws one workload's panel body.
func Detail(c *ui.Context, t *ui.Theme, item Row, now time.Time) {
	color := statusColor(t, item.Status)
	ui.Row(c).Padding(10, 12).Gap(9).AlignItems(ui.Center).Radius(8).Background(kit.Tint(t, color)).Children(func() {
		ui.Box(c).Size(9, 9).Radius(5).Background(color).Shrink(0)
		ui.Text(c, item.Status.Label()).Bold().TextColor(color).Grow(1)
		ui.Text(c, item.ReadyText()).Bold()
	})

	created := "—"
	if !item.Created.IsZero() {
		created = item.Created.Format("Jan 2, 2006 15:04")
	}
	detailSection(c, t, "Overview", []kit.Fact{
		{Label: "Namespace", Value: item.Namespace},
		{Label: "Kind", Value: item.Kind},
		{Label: "Created", Value: created},
		{Label: "Age", Value: kit.Age(item.Created, now)},
	})
	detailSection(c, t, "Capacity", []kit.Fact{
		{Label: "Ready", Value: fmt.Sprintf("%d", item.Ready)},
		{Label: "Desired", Value: desiredText(item.Desired)},
	})
}

func detailSection(c *ui.Context, t *ui.Theme, title string, facts []kit.Fact) {
	ui.Column(c).Gap(0).Radius(8).Clip().Border(1, t.Border).Children(func() {
		ui.Text(c, title).Padding(9, 12).FontSize(12).Bold().TextColor(t.TextMuted).Background(t.SurfaceHover)
		for i, fact := range facts {
			fact := fact
			row := ui.Row(c).Padding(9, 12).Gap(10).AlignItems(ui.Start)
			if i < len(facts)-1 {
				row.BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
			}
			row.Children(func() {
				ui.Text(c, fact.Label).Width(96).Shrink(0).TextColor(t.TextMuted)
				value := fact.Value
				if value == "" {
					value = "—"
				}
				text := ui.Text(c, value).Grow(1).MinWidth(0)
				if fact.Tone != kit.ToneNone {
					text.TextColor(fact.Tone.Color(t))
				}
			})
		}
	})
}

func desiredText(desired int32) string {
	if desired <= 0 {
		return "—"
	}
	return fmt.Sprintf("%d", desired)
}

func statusColor(t *ui.Theme, status Status) ui.Color {
	switch status {
	case StatusHealthy, StatusSucceeded, StatusScheduled:
		return t.Success
	case StatusActive, StatusSuspended:
		return t.Warning
	case StatusFailed:
		return t.Danger
	default:
		return t.TextMuted
	}
}

func statusTone(status Status) kit.Tone {
	switch status {
	case StatusHealthy, StatusSucceeded, StatusScheduled:
		return kit.ToneOK
	case StatusActive, StatusSuspended:
		return kit.ToneWarn
	case StatusFailed:
		return kit.ToneBad
	default:
		return kit.ToneMuted
	}
}

func nonEmptyKinds(kinds []KindSummary) int {
	count := 0
	for _, kind := range kinds {
		if kind.Total > 0 {
			count++
		}
	}
	return count
}
