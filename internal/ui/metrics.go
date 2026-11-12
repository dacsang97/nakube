package ui

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// Trace is one series of a metrics chart. Every series has the same
// slots. Line false is drawn as bars, true as a line over them.
type Trace struct {
	Name   string
	Color  ui.Color
	Points []Sample
	Line   bool
}

// Cursor is the hover rule and marker.
var Cursor = ui.Hex("#f5a623")

// MetricsHigh is the top of the value axis: the highest real value with
// headroom, so a flat series does not sit on the top edge. No value is 1.
func MetricsHigh(series []Trace) float64 {
	hi := 0.0
	for _, s := range series {
		for _, p := range s.Points {
			if p.OK && p.Value > hi {
				hi = p.Value
			}
		}
	}
	if hi <= 0 {
		return 1
	}
	return hi * 1.1
}

// MetricsChart draws series over shared slots:
// values on the right, times below, grid lines across. hover is the
// chosen slot, or -1: the pointer and the arrows move it, and it shows an
// orange rule, a diamond on the first bar series, and the slot's values.
// A gap draws nothing in its slot, and a line breaks over it.
func MetricsChart(c *ui.Context, t *ui.Theme, series []Trace, format func(float64) string, hover *int) {
	n := slots(series)
	if n == 0 {
		return
	}
	times := series[0].Points
	span := times[n-1].At.Sub(times[0].At)
	labels := make([]string, n)
	x := chart.NewBand(labels...).Padding(0.3, 0.1)
	y := chart.NewLinear(0, MetricsHigh(series)).Nice(4)
	y.Format = format
	yTicks := y.Ticks(4)
	xs := make([]float32, n)
	for i := range xs {
		xs[i] = x.At(i)
	}
	// Four time labels remain legible in the narrow detail panel.
	every := max(1, (n+3)/4)
	var bottom []chart.Tick
	for i := 0; i < n; i += every {
		bottom = append(bottom, chart.Tick{At: xs[i], Label: clock(times[i].At, span)})
	}
	grid := Track(t)
	chart.Plot(c, chart.Axes{
		Right:  yTicks,
		Bottom: bottom,
		Label: func(_ chart.Side, tick chart.Tick) {
			ui.Text(c, tick.Label).NoWrap().TextColor(t.TextMuted).FontSize(11).FontFeatures("tnum")
		},
	}, func(area ui.Element) {
		area.Draw(func(p *ui.Painter, r ui.Rect) {
			for _, tick := range yTicks {
				gy := chart.Y(r, tick.At)
				p.Line(r.X, gy, r.X+r.W, gy, 1, grid)
			}
			for _, s := range series {
				if !s.Line {
					continue
				}
				ys := make([]float32, n)
				for i := range ys {
					ys[i] = float32(math.NaN())
					if i < len(s.Points) && s.Points[i].OK {
						ys[i] = y.At(s.Points[i].Value)
					}
				}
				p.StrokePath(chart.Line(r, xs, ys, chart.CurveLinear), 1.5, s.Color)
			}
		})
		for _, s := range series {
			if s.Line {
				continue
			}
			for i, pt := range s.Points {
				if i >= n || !pt.OK || pt.Value <= 0 {
					continue
				}
				chart.Rect(c, x.Band(i), y.Span(0, pt.Value)).Background(s.Color).Radius(1)
			}
		}
		chart.Select(c, area, hover, xs, func(i int) string {
			return slotText(series, i, span, format)
		})
		if i := *hover; i >= 0 && i < n {
			chart.RuleX(c, xs[i], 2).Background(Cursor)
			if v, ok := primary(series, i); ok {
				chart.Point(c, xs[i], y.At(v), 8, 8).Rotate(45).Background(t.Background).Border(2, Cursor)
			}
			h := ui.Start
			if i > n/2 {
				h = ui.End
			}
			chart.Anchor(c, xs[i], 1, h, ui.Start, func() {
				tooltip(c, t, series, i, span, format)
			})
		}
	}).Height(160).Gap(6)
}

// MetricsLegend lists the series in a row under the chart.
func MetricsLegend(c *ui.Context, t *ui.Theme, series []Trace) {
	ui.Row(c).Gap(14).Justify(ui.Center).Wrap().Children(func() {
		for _, s := range series {
			color, name := s.Color, s.Name
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(2).Shrink(0).Background(color)
				ui.Text(c, name).TextColor(t.TextMuted).FontSize(12).SingleLine()
			})
		}
	})
}

func slots(series []Trace) int {
	if len(series) == 0 {
		return 0
	}
	return len(series[0].Points)
}

// primary is the first bar series' value in slot i, where the marker goes.
func primary(series []Trace, i int) (float64, bool) {
	for _, s := range series {
		if !s.Line && i < len(s.Points) {
			return s.Points[i].Value, s.Points[i].OK
		}
	}
	return 0, false
}

func tooltip(c *ui.Context, t *ui.Theme, series []Trace, i int, span time.Duration, format func(float64) string) {
	ui.Column(c).Gap(4).Padding(6, 8).Margin(4, 6, 0, 6).Radius(6).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, clock(series[0].Points[i].At, span)).NoWrap().Bold().FontSize(12).FontFeatures("tnum")
		for _, s := range series {
			if i >= len(s.Points) {
				continue
			}
			color, name, value := s.Color, s.Name, valueText(s.Points[i], format)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(2).Shrink(0).Background(color)
				ui.Text(c, name+"  "+value).NoWrap().FontSize(12).FontFeatures("tnum")
			})
		}
	})
}

func slotText(series []Trace, i int, span time.Duration, format func(float64) string) string {
	text := clock(series[0].Points[i].At, span)
	for _, s := range series {
		if i < len(s.Points) {
			text += ", " + s.Name + " " + valueText(s.Points[i], format)
		}
	}
	return text
}

// valueText is a gap's em dash, never a zero.
func valueText(p Sample, format func(float64) string) string {
	if !p.OK {
		return "—"
	}
	if format == nil {
		return strconv.FormatFloat(p.Value, 'g', 3, 64)
	}
	return format(p.Value)
}
