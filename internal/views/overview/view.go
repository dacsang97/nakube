package overview

import (
	"time"

	"github.com/egoist/mygo/ui"

	kit "github.com/dacsang97/nakube/internal/ui"
)

// Wait says which readings have not arrived. A nil pointer is a settled screen.
// Facts is every cluster fact still out. Nodes, Pods, and Events wait on their
// own list. Metrics waits on usage only. A ready reading is drawn; the others
// stay skeletons.
type Wait struct {
	Facts   bool
	Nodes   bool
	Pods    bool
	Events  bool
	Metrics bool
}

// Draw is the cluster overview. Role, metric, and window persist through
// the pointers. A nil pointer keeps the choice for this frame only.
func Draw(c *ui.Context, t *ui.Theme, snap Snapshot, samples []Sample, contextName string, role, metric, window *string, now time.Time, onOpen func(string), wait *Wait) {
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
	facts, metrics := false, false
	nodes, pods, events := false, false, false
	if wait != nil {
		facts, metrics = wait.Facts, wait.Metrics
		nodes, pods, events = wait.Nodes, wait.Pods, wait.Events
		if facts {
			nodes, pods, events = true, true, true
		}
	}
	ui.Scroll(c).Grow(1).FillHeight().Scrollbars(false).Children(func() {
		ui.Column(c).Gap(14).Children(func() {
			header(c, t, snap, contextName, shown, role, metric, window, nodes, pods)
			if nodes && pods && events {
				loadingBody(c, t, shown, onOpen)
				return
			}
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				if nodes || pods {
					cardSkeleton(c, t, "CPU", colors.CPU, true)
					cardSkeleton(c, t, "Memory", colors.Memory, true)
				} else {
					metricCard(c, t, "CPU", colors.CPU, shown.CPU, shown.CPUHist, true, metrics)
					metricCard(c, t, "Memory", colors.Memory, shown.Memory, shown.MemHist, false, metrics)
				}
				if pods {
					cardSkeleton(c, t, "Pods", colors.Pods, false)
				} else {
					podCard(c, t, colors.Pods, shown)
				}
			})
			historyCard(c, t, shown, colors, metrics)
			if nodes || pods {
				donutSkeletonRow(c, t, colors, pods)
			} else {
				donutRow(c, t, shown, colors, metrics)
			}
			if events {
				messageSkeleton(c, t, onOpen)
			} else {
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
			}
		})
	})
}

func header(c *ui.Context, t *ui.Theme, snap Snapshot, contextName string, shown Presented, role, metric, window *string, nodes, pods bool) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Column(c).Grow(1).MinWidth(160).Gap(2).Children(func() {
			ui.Text(c, "Overview").FontSize(22).Bold()
			if nodes && pods {
				kit.Bar(c, t, 220, 12)
			} else {
				countLine(c, t, snap, contextName, nodes, pods)
			}
		})
		filters(c, shown, role, metric, window)
	})
}

func countLine(c *ui.Context, t *ui.Theme, snap Snapshot, contextName string, nodes, pods bool) {
	name := contextName
	if name == "" {
		name = "No context"
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, name).TextColor(t.TextMuted)
		ui.Text(c, "·").TextColor(t.TextMuted)
		if nodes {
			kit.Bar(c, t, 72, 12)
		} else {
			ui.Text(c, snap.NodesText()+" nodes").TextColor(t.TextMuted)
		}
		ui.Text(c, "·").TextColor(t.TextMuted)
		if pods {
			kit.Bar(c, t, 72, 12)
		} else {
			ui.Text(c, snap.PodsText()+" pods").TextColor(t.TextMuted)
		}
	})
}

func donutSkeletonRow(c *ui.Context, t *ui.Theme, colors kit.Series, pods bool) {
	kit.Panel(c, t).Children(func() {
		ui.Row(c).AlignItems(ui.Stretch).Children(func() {
			donutSkeleton(c, t, "CPU", colors.CPU)
			split(c, t)
			donutSkeleton(c, t, "Memory", colors.Memory)
			split(c, t)
			donutSkeleton(c, t, "Pods", colors.Pods)
		})
	})
	_ = pods
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

func metricCard(c *ui.Context, t *ui.Theme, title string, color ui.Color, series Series, hist []float64, cpu, metrics bool) {
	kit.Panel(c, t).Grow(1).Children(func() {
		kit.Title(c, t, nil, title, color)
		if metrics && !series.UsageOK {
			ui.Text(c, "Now").TextColor(t.TextMuted).FontSize(12)
			kit.Status(c, "Loading metrics").Children(func() {
				kit.Bar(c, t, 72, 28)
			})
		} else {
			caption, hero := heroText(series, cpu)
			ui.Text(c, caption).TextColor(t.TextMuted).FontSize(12)
			size := float32(32)
			if hero == "metrics unavailable" {
				size = 16
			}
			ui.Text(c, hero).FontSize(size).Bold().NoWrap()
		}
		ui.Row(c).Gap(12).Children(func() {
			stat(c, t, "Requests", amount(series.Requests, cpu))
			stat(c, t, "Limits", amount(series.Limits, cpu))
			stat(c, t, "Allocatable", amount(series.Allocatable, cpu))
		})
		if metrics && !series.UsageOK {
			kit.Block(c, t, 58, 12)
		} else {
			kit.Dots(c, t, hist, histMax(hist), color)
		}
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

func historyCard(c *ui.Context, t *ui.Theme, shown Presented, colors kit.Series, metrics bool) {
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
			if metrics {
				kit.Status(c, "Loading metrics").Children(func() {
					kit.Block(c, t, 148, 12)
				})
				return
			}
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
		bars := make([]kit.Sample, len(shown.History))
		for i, p := range shown.History {
			bars[i] = kit.Sample{At: p.At, Value: p.Value, OK: p.OK}
		}
		kit.Bars(c, t, shown.Start, shown.End, bars, hi, color, format)
		kit.Legend(c, t, []kit.Swatch{{Color: color, Name: "Usage", Value: format(lastValue(shown.History))}})
	})
}

func donutRow(c *ui.Context, t *ui.Theme, shown Presented, colors kit.Series, metrics bool) {
	kit.Panel(c, t).Children(func() {
		ui.Row(c).AlignItems(ui.Stretch).Children(func() {
			donutCol(c, t, "CPU", colors.CPU, Percent(shown.CPU.Usage, shown.CPU.Allocatable, shown.CPU.UsageOK), "usage", rings(shown.CPU, colors), legend(shown.CPU, true, colors), shown.CPU.OverLimit, metrics && !shown.CPU.UsageOK)
			split(c, t)
			donutCol(c, t, "Memory", colors.Memory, Percent(shown.Memory.Usage, shown.Memory.Allocatable, shown.Memory.UsageOK), "usage", rings(shown.Memory, colors), legend(shown.Memory, false, colors), shown.Memory.OverLimit, metrics && !shown.Memory.UsageOK)
			split(c, t)
			donutCol(c, t, "Pods", colors.Pods, itoa(int(shown.Pods.Usage)), "pods", []kit.Ring{{
				Value: shown.Pods.Usage, Total: shown.Pods.Allocatable, Color: colors.Requests, Known: shown.Pods.Allocatable > 0 || shown.Pods.UsageOK,
			}}, []kit.Swatch{
				{Color: colors.Requests, Name: "Usage", Value: itoa(int(shown.Pods.Usage))},
				{Color: colors.Allocatable, Name: "Allocatable", Value: itoa(int(shown.Pods.Allocatable))},
				{Color: colors.Capacity, Name: "Capacity", Value: itoa(int(shown.Pods.Capacity))},
			}, false, false)
		})
	})
}

func donutCol(c *ui.Context, t *ui.Theme, title string, accent ui.Color, center, caption string, rings []kit.Ring, items []kit.Swatch, warn, hold bool) {
	ui.Column(c).Grow(1).MinWidth(0).Gap(12).Padding(0, 8).Children(func() {
		kit.Title(c, t, nil, title, accent)
		ui.Row(c).Justify(ui.Center).Children(func() {
			centerText, captionText := center, caption
			if hold {
				centerText, captionText = "", ""
			}
			ui.Box(c).Size(96, 96).Shrink(0).Children(func() {
				kit.Donut(c, t, 96, centerText, captionText, rings)
				if hold {
					ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						kit.Status(c, "Loading metrics").Children(func() {
							kit.Bar(c, t, 36, 14)
						})
					})
				}
			})
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

func loadingBody(c *ui.Context, t *ui.Theme, shown Presented, onOpen func(string)) {
	colors := kit.Colors()
	kit.Status(c, "Loading").Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
			cardSkeleton(c, t, "CPU", colors.CPU, true)
			cardSkeleton(c, t, "Memory", colors.Memory, true)
			cardSkeleton(c, t, "Pods", colors.Pods, false)
		})
		kit.Panel(c, t).Children(func() {
			title, color := "CPU", colors.CPU
			if shown.Metric == Memory {
				title, color = "Memory", colors.Memory
			}
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				kit.Title(c, t, nil, title, color)
				kit.Bar(c, t, 180, 12)
			})
			kit.Block(c, t, 148, 12)
		})
		kit.Panel(c, t).Children(func() {
			ui.Row(c).AlignItems(ui.Stretch).Children(func() {
				donutSkeleton(c, t, "CPU", colors.CPU)
				split(c, t)
				donutSkeleton(c, t, "Memory", colors.Memory)
				split(c, t)
				donutSkeleton(c, t, "Pods", colors.Pods)
			})
		})
		messageSkeleton(c, t, onOpen)
	})
}

func cardSkeleton(c *ui.Context, t *ui.Theme, title string, color ui.Color, chart bool) {
	kit.Panel(c, t).Grow(1).MinWidth(0).Children(func() {
		kit.Title(c, t, nil, title, color)
		kit.Bar(c, t, 120, 12)
		kit.Bar(c, t, 64, 28)
		ui.Row(c).Gap(12).Children(func() {
			kit.Bar(c, t, 48, 12)
			kit.Bar(c, t, 48, 12)
			kit.Bar(c, t, 48, 12)
		})
		if chart {
			kit.Block(c, t, 58, 12)
		}
	})
}

func donutSkeleton(c *ui.Context, t *ui.Theme, title string, color ui.Color) {
	ui.Column(c).Grow(1).MinWidth(0).Gap(12).Padding(0, 8).Children(func() {
		kit.Title(c, t, nil, title, color)
		ui.Row(c).Justify(ui.Center).Children(func() {
			kit.Circle(c, t, 96)
		})
		kit.Bar(c, t, 0, 12)
		kit.Bar(c, t, 0, 12)
		kit.Bar(c, t, 0, 12)
	})
}

func messageSkeleton(c *ui.Context, t *ui.Theme, onOpen func(string)) {
	kit.Panel(c, t).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "Messages").FontSize(16).Bold()
			kit.Bar(c, t, 88, 12)
			if onOpen != nil && ui.Button(c, "Events").Clicked() {
				onOpen("events")
			}
		})
		for range 3 {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(t.Border)
				kit.Bar(c, t, 120, 12)
				kit.Bar(c, t, 0, 12)
				kit.Bar(c, t, 40, 12)
			})
		}
	})
}
