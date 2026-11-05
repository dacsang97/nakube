package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// Bar is one real sample in a history. OK false is a gap, not a zero bar.
type Bar struct {
	At    time.Time
	Value float64
	OK    bool
}

// Bars draws usage over a window. No numeric axis: the shape is the point,
// and the tooltip carries the value. Empty input draws nothing.
func Bars(c *ui.Context, t *ui.Theme, start, end time.Time, bars []Bar, max float64, color ui.Color, format func(float64) string) {
	if !end.After(start) || len(bars) == 0 {
		return
	}
	if max <= 0 {
		max = 1
	}
	y := chart.NewLinear(0, max)
	x := chart.NewLinear(float64(start.Unix()), float64(end.Unix()))
	span := end.Sub(start)
	ticks := []chart.Tick{
		{At: 0, Label: clock(start, span)},
		{At: 1, Label: "now"},
	}
	well := Tint(t, color)
	step := span / time.Duration(len(bars))
	if step <= 0 {
		step = time.Second
	}
	half := float64(step) / float64(time.Second) * 0.36
	chart.Plot(c, chart.Axes{
		Bottom: ticks,
		Label: func(_ chart.Side, tick chart.Tick) {
			ui.Text(c, tick.Label).NoWrap().TextColor(t.TextMuted).FontSize(11)
		},
	}, func(area ui.Element) {
		area.Background(well).Radius(12)
		for _, bar := range bars {
			if !bar.OK {
				continue
			}
			at := float64(bar.At.Unix())
			spanX := chart.Span{Lo: x.At(at - half), Hi: x.At(at + half)}
			tip := clock(bar.At, end.Sub(start))
			if format != nil {
				tip += "  " + format(bar.Value)
			}
			chart.Rect(c, spanX, y.Span(0, bar.Value)).Background(color).Radius(3).Tooltip(tip)
		}
	}).Height(148).Gap(4)
}

func clock(at time.Time, span time.Duration) string {
	if span <= 6*time.Hour {
		return at.Format("15:04")
	}
	return at.Format("Jan 2 15:04")
}
