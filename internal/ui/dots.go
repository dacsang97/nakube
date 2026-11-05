package ui

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Dots draws a history: a column of dots per bucket, filled
// from the bottom up to the value. A NaN bucket stays faint. Faint dots
// are the well, not a measured zero.
func Dots(c *ui.Context, t *ui.Theme, values []float64, max float64, color ui.Color) {
	well := Tint(t, color)
	ui.Box(c).Height(58).Radius(12).Background(well).Draw(func(p *ui.Painter, r ui.Rect) {
		paintDots(p, r, values, max, color, t)
	})
}

func paintDots(p *ui.Painter, r ui.Rect, values []float64, max float64, color ui.Color, t *ui.Theme) {
	const rows = 6
	cols := len(values)
	if cols == 0 {
		return
	}
	pad := float32(8)
	w := r.W - 2*pad
	h := r.H - 2*pad
	if w <= 0 || h <= 0 {
		return
	}
	pitchX := w / float32(cols)
	pitchY := h / float32(rows)
	radius := float32(math.Min(float64(pitchX), float64(pitchY)) * 0.28)
	if radius < 1.1 {
		radius = 1.1
	}
	faint := Track(t)
	for col, v := range values {
		filled := 0
		if !math.IsNaN(v) && max > 0 && v > 0 {
			filled = int(math.Round(v / max * float64(rows)))
			if filled < 1 {
				filled = 1
			}
			if filled > rows {
				filled = rows
			}
		}
		cx := r.X + pad + pitchX*(float32(col)+0.5)
		for row := 0; row < rows; row++ {
			cy := r.Y + pad + pitchY*(float32(rows-1-row)+0.5)
			dot := color
			if row >= filled {
				dot = faint
			}
			var path ui.Path
			path.Circle(cx, cy, radius)
			p.FillPath(&path, dot)
		}
	}
}
