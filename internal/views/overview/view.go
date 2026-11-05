package overview

import (
	"time"

	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

// Draw is the cluster overview. Role, metric, and window persist through
// the pointers. A nil pointer keeps the choice for this frame only.
func Draw(c *ui.Context, t *ui.Theme, snap Snapshot, samples []Sample, contextName string, role, metric, window *string, now time.Time, onOpen func(string)) {
	if now.IsZero() {
		now = time.Now()
	}
	roleV, metricV, windowV := Worker, CPU, "15m"
	if role != nil && *role != "" {
		roleV = *role
	}
	if metric != nil && *metric != "" {
		metricV = *metric
	}
	if window != nil && *window != "" {
		windowV = *window
	}
	shown := Present(snap, roleV, metricV, windowV, samples, now)
	if role != nil {
		*role = shown.Role
	}
	colors := kit.Colors()
	ui.Scroll(c).Grow(1).FillHeight().Scrollbars(false).Children(func() {
		ui.Column(c).Gap(14).Children(func() {
			header(c, t, snap, contextName, shown, role, metric, window)
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				metricCard(c, t, "CPU", colors.CPU, shown.CPU, shown.CPUHist, true)
				metricCard(c, t, "Memory", colors.Memory, shown.Memory, shown.MemHist, false)
				podCard(c, t, colors.Pods, shown)
			})
			historyCard(c, t, shown, colors)
			donutRow(c, t, shown, colors)
			kit.Panel(c, t).Children(func() {
				rows := make([]kit.Message, len(shown.Events))
				for i, ev := range shown.Events {
					rows[i] = kit.Message{
						Reason: ev.Reason,
						Object: ev.Object,
						Body:   ev.Message,
						Age:    Age(ev.When, now),
					}
				}
				var open func()
				if onOpen != nil {
					open = func() { onOpen("events") }
				}
				kit.Messages(c, t, snap.Warnings, rows, open)
			})
		})
	})
}

func header(c *ui.Context, t *ui.Theme, snap Snapshot, contextName string, shown Presented, role, metric, window *string) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Column(c).Grow(1).MinWidth(160).Gap(2).Children(func() {
			ui.Text(c, "Overview").FontSize(22).Bold()
			ui.Text(c, subtitle(snap, contextName)).TextColor(t.TextMuted)
		})
		filters(c, shown, role, metric, window)
	})
}

func subtitle(snap Snapshot, contextName string) string {
	name := contextName
	if name == "" {
		name = "No context"
	}
	line := name + "  ·  " + snap.NodesText() + " nodes  ·  " + snap.PodsText() + " pods"
	if len(snap.NotReady) > 0 {
		line += "  ·  " + snap.NotReady[0] + " not ready"
	}
	return line
}

func filters(c *ui.Context, shown Presented, role, metric, window *string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Shrink(0).Children(func() {
		roleLabels := roleChoices(shown)
		roleIndex := indexOf(roleLabels, roleLabel(shown.Role))
		ui.Segmented(c, &roleIndex, roleLabels...).Label("Nodes")
		if role != nil && roleIndex >= 0 && roleIndex < len(roleLabels) {
			if roleLabels[roleIndex] == "Master" {
				*role = Master
			} else {
				*role = Worker
			}
		}
		metricLabels := []string{"CPU", "Memory"}
		metricIndex := 0
		if shown.Metric == Memory {
			metricIndex = 1
		}
		ui.Segmented(c, &metricIndex, metricLabels...).Label("Metric")
		if metric != nil {
			if metricIndex == 1 {
				*metric = Memory
			} else {
				*metric = CPU
			}
		}
		labels := make([]string, len(Windows()))
		for i, w := range Windows() {
			labels[i] = w.Label
		}
		winIndex := indexOf(labels, shown.Window.Label)
		ui.Segmented(c, &winIndex, labels...).Label("Time range")
		if window != nil && winIndex >= 0 && winIndex < len(labels) {
			*window = labels[winIndex]
		}
	})
}

func roleChoices(shown Presented) []string {
	switch {
	case shown.HasMaster && shown.HasWorker:
		return []string{"Worker", "Master"}
	case shown.HasMaster:
		return []string{"Master"}
	default:
		return []string{"Worker"}
	}
}

func roleLabel(role string) string {
	if role == Master {
		return "Master"
	}
	return "Worker"
}

func indexOf(labels []string, want string) int {
	for i, label := range labels {
		if label == want {
			return i
		}
	}
	return 0
}

func metricCard(c *ui.Context, t *ui.Theme, title string, color ui.Color, series Series, hist []float64, cpu bool) {
	kit.Panel(c, t).Grow(1).Children(func() {
		kit.Title(c, t, nil, title, color)
		caption, hero := heroText(series, cpu)
		ui.Text(c, caption).TextColor(t.TextMuted).FontSize(12)
		size := float32(32)
		if hero == "metrics unavailable" {
			size = 16
		}
		ui.Text(c, hero).FontSize(size).Bold().NoWrap()
		ui.Row(c).Gap(12).Children(func() {
			stat(c, t, "Requests", amount(series.Requests, cpu))
			stat(c, t, "Limits", amount(series.Limits, cpu))
			stat(c, t, "Allocatable", amount(series.Allocatable, cpu))
		})
		kit.Dots(c, t, hist, histMax(hist), color)
	})
}

func podCard(c *ui.Context, t *ui.Theme, color ui.Color, shown Presented) {
	kit.Panel(c, t).Grow(1).Children(func() {
		kit.Title(c, t, nil, "Pods", color)
		ui.Text(c, "Scheduled on these nodes").TextColor(t.TextMuted).FontSize(12)
		ui.Text(c, itoa(shown.Running)).FontSize(32).Bold()
		ui.Row(c).Gap(12).Children(func() {
			stat(c, t, "Pending", itoa(shown.Pending))
			stat(c, t, "Failed", itoa(shown.Failed))
			stat(c, t, "Capacity", itoa(int(shown.Pods.Capacity)))
		})
	})
}

func historyCard(c *ui.Context, t *ui.Theme, shown Presented, colors kit.Series) {
	title := "CPU"
	color := colors.CPU
	format := FormatMilli
	if shown.Metric == Memory {
		title = "Memory"
		color = colors.Memory
		format = FormatBytes
	}
	kit.Panel(c, t).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			kit.Title(c, t, nil, title, color)
			ui.Text(c, "Usage  ·  "+shown.Window.Label+"  ·  "+roleLabel(shown.Role)+" nodes").TextColor(t.TextMuted).Grow(1)
		})
		if !anyPoint(shown.History) {
			if shown.MetricsOK {
				ui.Text(c, "No samples in this range").TextColor(t.TextMuted)
			} else {
				ui.Text(c, "metrics unavailable").TextColor(t.TextMuted)
			}
			return
		}
		hi := 1.0
		for _, p := range shown.History {
			if p.OK && p.Value > hi {
				hi = p.Value
			}
		}
		hi *= 1.15
		bars := make([]kit.Bar, len(shown.History))
		for i, p := range shown.History {
			bars[i] = kit.Bar{At: p.At, Value: p.Value, OK: p.OK}
		}
		kit.Bars(c, t, shown.Start, shown.End, bars, hi, color, format)
		kit.Legend(c, t, []kit.Swatch{{Color: color, Name: "Usage", Value: format(lastValue(shown.History))}})
	})
}

func donutRow(c *ui.Context, t *ui.Theme, shown Presented, colors kit.Series) {
	kit.Panel(c, t).Children(func() {
		ui.Row(c).AlignItems(ui.Stretch).Children(func() {
			donutCol(c, t, "CPU", colors.CPU, Percent(shown.CPU.Usage, shown.CPU.Allocatable, shown.CPU.UsageOK), "usage", rings(shown.CPU, colors), legend(shown.CPU, true, colors), shown.CPU.OverLimit)
			split(c, t)
			donutCol(c, t, "Memory", colors.Memory, Percent(shown.Memory.Usage, shown.Memory.Allocatable, shown.Memory.UsageOK), "usage", rings(shown.Memory, colors), legend(shown.Memory, false, colors), shown.Memory.OverLimit)
			split(c, t)
			donutCol(c, t, "Pods", colors.Pods, itoa(int(shown.Pods.Usage)), "pods", []kit.Ring{{
				Value: shown.Pods.Usage, Total: shown.Pods.Allocatable, Color: colors.Requests, Known: shown.Pods.Allocatable > 0 || shown.Pods.UsageOK,
			}}, []kit.Swatch{
				{Color: colors.Requests, Name: "Usage", Value: itoa(int(shown.Pods.Usage))},
				{Color: colors.Allocatable, Name: "Allocatable", Value: itoa(int(shown.Pods.Allocatable))},
				{Color: colors.Capacity, Name: "Capacity", Value: itoa(int(shown.Pods.Capacity))},
			}, false)
		})
	})
}

func donutCol(c *ui.Context, t *ui.Theme, title string, accent ui.Color, center, caption string, rings []kit.Ring, items []kit.Swatch, warn bool) {
	ui.Column(c).Grow(1).MinWidth(0).Gap(12).Padding(0, 8).Children(func() {
		kit.Title(c, t, nil, title, accent)
		ui.Row(c).Justify(ui.Center).Children(func() {
			kit.Donut(c, t, 96, center, caption, rings)
		})
		kit.Legend(c, t, items)
		if warn {
			ui.Text(c, "Limits exceed allocatable").TextColor(t.Danger).FontSize(12).SingleLine()
		}
	})
}

func split(c *ui.Context, t *ui.Theme) {
	ui.Box(c).Width(1).Shrink(0).Margin(4, 16).Background(t.Border)
}

func rings(series Series, colors kit.Series) []kit.Ring {
	return []kit.Ring{
		{Value: series.Usage, Total: series.Allocatable, Color: colors.Usage, Known: series.UsageOK},
		{Value: series.Requests, Total: series.Allocatable, Color: colors.Requests, Known: true},
		{Value: series.Limits, Total: series.Allocatable, Color: colors.Limits, Known: true},
	}
}

func legend(series Series, cpu bool, colors kit.Series) []kit.Swatch {
	usage := "—"
	if series.UsageOK {
		usage = amount(series.Usage, cpu)
	}
	return []kit.Swatch{
		{Color: colors.Usage, Name: "Usage", Value: usage},
		{Color: colors.Requests, Name: "Requests", Value: amount(series.Requests, cpu)},
		{Color: colors.Limits, Name: "Limits", Value: amount(series.Limits, cpu)},
		{Color: colors.Allocatable, Name: "Allocatable", Value: amount(series.Allocatable, cpu)},
		{Color: colors.Capacity, Name: "Capacity", Value: amount(series.Capacity, cpu)},
	}
}

func heroText(series Series, cpu bool) (caption, hero string) {
	if !series.UsageOK {
		return "Now", "metrics unavailable"
	}
	return amount(series.Usage, cpu) + " of " + amount(series.Allocatable, cpu), Percent(series.Usage, series.Allocatable, true)
}

func amount(v float64, cpu bool) string {
	if cpu {
		return FormatMilli(v)
	}
	return FormatBytes(v)
}

func stat(c *ui.Context, t *ui.Theme, name, value string) {
	ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
		ui.Text(c, name).FontSize(11).TextColor(t.TextMuted).NoWrap()
		ui.Text(c, value).Bold().NoWrap()
	})
}

func histMax(values []float64) float64 {
	var max float64
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return 1
	}
	return max
}

func anyPoint(points []Point) bool {
	for _, p := range points {
		if p.OK {
			return true
		}
	}
	return false
}

func lastValue(points []Point) float64 {
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].OK {
			return points[i].Value
		}
	}
	return 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
