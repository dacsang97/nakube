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

// Bars draws one slot per sample across the window. Slots stay equal and
// inset, so a sample near the end cannot slide its bar past the plot.
// OK false is a gap in that slot, not a zero bar. Empty input draws nothing.
func Bars(c *ui.Context, t *ui.Theme, start, end time.Time, bars []Bar, max float64, color ui.Color, format func(float64) string) {
	if !end.After(start) || len(bars) == 0 {
		return
	}
	if max <= 0 {
		max = 1
	}
	y := chart.NewLinear(0, max)
	slots := make([]string, len(bars))
	x := chart.NewBand(slots...).Padding(0.32, 0.55)
	span := end.Sub(start)
	well := Tint(t, color)
	chart.Plot(c, chart.Axes{
		Bottom: []chart.Tick{
			{At: 0, Label: clock(start, span)},
			{At: 1, Label: "now"},
		},
		Label: func(_ chart.Side, tick chart.Tick) {
			ui.Text(c, tick.Label).NoWrap().TextColor(t.TextMuted).FontSize(11)
		},
	}, func(area ui.Element) {
		area.Background(well).Radius(12)
		for i, bar := range bars {
			if !bar.OK || bar.Value <= 0 {
				continue
			}
			tip := clock(bar.At, span)
			if format != nil {
				tip += "  " + format(bar.Value)
			}
			chart.Rect(c, x.Band(i), y.Span(0, bar.Value)).Background(color).Radius(3).Tooltip(tip)
		}
	}).Height(148).Gap(4)
}

func clock(at time.Time, span time.Duration) string {
	if span <= 6*time.Hour {
		return at.Format("15:04")
	}
	return at.Format("Jan 2 15:04")
}
