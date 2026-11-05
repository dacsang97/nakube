package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// Ring is one donut band. Known false skips the colored arc and keeps the
// track, so a missing metric is not drawn as zero.
type Ring struct {
	Value float64
	Total float64
	Color ui.Color
	Known bool
}

// Donut draws concentric rings around a center label. Rings start at the
// outside. Overlap is the point: usage, requests, and limits are not slices
// of one pie.
func Donut(c *ui.Context, t *ui.Theme, size float32, center, caption string, rings []Ring) {
	ui.Box(c).Size(size, size).Shrink(0).Children(func() {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			paintRings(p, r, rings, Track(t))
		})
		ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).AlignItems(ui.Center).Justify(ui.Center).Gap(0).Children(func() {
			ui.Text(c, center).FontSize(16).Bold().NoWrap()
			if caption != "" {
				ui.Text(c, caption).FontSize(11).TextColor(t.TextMuted).NoWrap()
			}
		})
	})
}

func paintRings(p *ui.Painter, r ui.Rect, rings []Ring, track ui.Color) {
	if len(rings) == 0 {
		return
	}
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	outer := min32(r.W, r.H)/2 - 1
	thickness := float32(11)
	gap := float32(4)
	full := chart.Pie([]float64{1}, 0)
	for i, ring := range rings {
		radius := outer - float32(i)*(thickness+gap)
		inner := radius - thickness
		if inner < 8 {
			break
		}
		if len(full) > 0 {
			p.FillPath(chart.Sector(cx, cy, inner, radius, full[0]), track)
		}
		if !ring.Known || ring.Total <= 0 || ring.Value <= 0 {
			continue
		}
		used := ring.Value
		rest := ring.Total - ring.Value
		if rest < 0 {
			used, rest = ring.Total, 0
		}
		slices := chart.Pie([]float64{used, rest}, 0)
		if len(slices) == 0 {
			continue
		}
		p.FillPath(chart.Sector(cx, cy, inner, radius, slices[0]), ring.Color)
	}
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
