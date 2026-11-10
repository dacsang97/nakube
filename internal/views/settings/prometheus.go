package settings

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// Prometheus is the metrics source of the open context. Draft is what the
// field holds; an empty one finds the service on its own.
type Prometheus struct {
	Context string
	Draft   *string
	Saved   string
	Err     string
}

// DrawMetrics is the Metrics page. save reports that the draft should be
// stored for the open context.
func DrawMetrics(c *ui.Context, t *ui.Theme, prom Prometheus) (save bool) {
	page(c, t, "Metrics", "Where resource charts read their history.", func() {
		card(c, t, func() {
			ui.Text(c, "Prometheus").FontSize(16).Bold()
			if prom.Context == "" || prom.Draft == nil {
				ui.Text(c, "Open a context to choose its Prometheus.").TextColor(t.TextMuted).Wrap()
				return
			}
			ui.Text(c, "Service for "+prom.Context+", reached through the API server. Leave empty to find it on its own.").TextColor(t.TextMuted).Wrap()
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, prom.Draft).Grow(1).MinWidth(0).Placeholder("namespace/service:port").Label("Prometheus service")
				b := ui.Button(c, "Save").Shrink(0)
				if strings.TrimSpace(*prom.Draft) == prom.Saved {
					b.Disabled(true)
				}
				save = b.Clicked()
			})
			if prom.Err != "" {
				ui.Text(c, prom.Err).TextColor(t.Danger).Wrap()
			}
		})
	})
	return save
}
