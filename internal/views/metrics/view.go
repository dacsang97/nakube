package metrics

import (
	"github.com/egoist/mygo/ui"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
)

// Icons are the tab glyphs. The desktop owns the icon set.
type Icons map[string]*ui.SVG

// Draw paints the block. tab, rng and hover are the reader's choices; the
// caller fetches again when tab or rng changes.
func Draw(c *ui.Context, t *ui.Theme, v View, tab, rng *string, hover *int, icons Icons) {
	ui.Column(c).Gap(0).Radius(8).Clip().Border(1, t.Border).Children(func() {
		ui.Text(c, "Metrics").Padding(9, 12).FontSize(12).Bold().TextColor(t.TextMuted).Background(t.SurfaceHover)
		ui.Column(c).Padding(10, 12).Gap(10).Children(func() {
			toolbar(c, t, tab, rng, hover, icons)
			if v.Source != "" {
				ui.Text(c, "Displaying metrics from Prometheus: "+v.Source).FontSize(12).TextColor(t.TextMuted)
			}
			body(c, t, v, hover)
		})
	})
}

func toolbar(c *ui.Context, t *ui.Theme, tab, rng *string, hover *int, icons Icons) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Select(c, rng, Ranges()).Width(84).Label("Range")
		ui.Box(c).Width(1).Height(20).Background(t.Border).Margin(0, 4, 0, 4)
		for _, id := range cluster.MetricTabs() {
			name := TabLabel(id)
			button := ui.Button(c, "").Size(28, 28).Padding(0).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Tooltip(name).Label(name)
			if id == *tab {
				button.Background(t.SurfacePressed).Border(1, t.Border).TextColor(t.Text)
			} else {
				button.Border(0, ui.Transparent).TextColor(t.TextMuted)
				if button.Hovered() {
					button.Background(t.SurfaceHover)
				}
			}
			button.Children(func() {
				ui.Icon(c, icons[id]).Size(16, 16)
			})
			if button.Clicked() && *tab != id {
				*tab = id
				*hover = -1
			}
		}
	})
}

func body(c *ui.Context, t *ui.Theme, v View, hover *int) {
	traces := Traces(v.Tab, v.Results)
	switch {
	case v.Err != "":
		ui.Text(c, v.Err).TextColor(t.TextMuted)
	case v.Loading && len(traces) == 0:
		kit.Status(c, "Loading metrics").Children(func() {
			kit.Block(c, t, 160, 6)
		})
	case len(traces) == 0:
		ui.Text(c, "No data in this range").TextColor(t.TextMuted)
	default:
		kit.MetricsChart(c, t, traces, Format(v.Tab), hover)
		kit.MetricsLegend(c, t, traces)
	}
}
