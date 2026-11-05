package desktop

import (
	"fmt"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/dacsang97/nakube/internal/cluster"
)

// Overview is the numbers the first screen draws. Metrics text is never an
// invented figure: a missing API is the words "metrics unavailable".
type Overview struct {
	NodesReady    int
	NodesTotal    int
	PodsRunning   int
	PodsPending   int
	PodsFailed    int
	PodsSucceeded int
	PodsUnknown   int
	PodsTotal     int
	Warnings      int
	CPU           string
	Memory        string
	MetricsOK     bool
	CPUMilli      int64
	MemoryBytes   int64
	CPUAlloc      int64
	MemAlloc      int64
	NotReady      []string
	NotReadyMore  int
	Events        []overviewEvent
}

type overviewEvent struct {
	When    time.Time
	Object  string
	Reason  string
	Message string
}

type phaseCount struct {
	Name  string
	Count int
	Color func(*ui.Theme) ui.Color
}

func (o Overview) NodesText() string {
	return fmt.Sprintf("%d/%d", o.NodesReady, o.NodesTotal)
}

func (o Overview) PodsText() string {
	return fmt.Sprintf("%d/%d", o.PodsRunning, o.PodsTotal)
}

func (o Overview) WarningsText() string {
	return fmt.Sprintf("%d", o.Warnings)
}

func (o Overview) phases() []phaseCount {
	return []phaseCount{
		{"Running", o.PodsRunning, func(t *ui.Theme) ui.Color { return t.Success }},
		{"Pending", o.PodsPending, func(t *ui.Theme) ui.Color { return t.Warning }},
		{"Failed", o.PodsFailed, func(t *ui.Theme) ui.Color { return t.Danger }},
		{"Succeeded", o.PodsSucceeded, func(t *ui.Theme) ui.Color { return t.Accent }},
		{"Unknown", o.PodsUnknown, func(t *ui.Theme) ui.Color { return t.TextMuted }},
	}
}

func (o Overview) cpuScale() (float64, float64, bool) {
	if !o.MetricsOK || o.CPUAlloc <= 0 {
		return 0, 1, false
	}
	return float64(o.CPUMilli), float64(o.CPUAlloc), true
}

func (o Overview) memScale() (float64, float64, bool) {
	if !o.MetricsOK || o.MemAlloc <= 0 {
		return 0, 1, false
	}
	return float64(o.MemoryBytes), float64(o.MemAlloc), true
}

// metricsCardText is what the CPU and memory cards show. NotFound and
// ServiceUnavailable say the API is missing. A real value is shown only
// when the metrics client returned one.
func metricsCardText(err error, available bool, value string) string {
	if apierrors.IsNotFound(err) || apierrors.IsServiceUnavailable(err) || !available {
		return "metrics unavailable"
	}
	if value == "" {
		return "metrics unavailable"
	}
	return value
}

func formatCPU(milli int64) string {
	return fmt.Sprintf("%dm", milli)
}

func formatBytes(n int64) string {
	const (
		ki = 1024
		mi = 1024 * ki
		gi = 1024 * mi
	)
	switch {
	case n >= gi:
		return fmt.Sprintf("%.1f Gi", float64(n)/float64(gi))
	case n >= mi:
		return fmt.Sprintf("%.1f Mi", float64(n)/float64(mi))
	case n >= ki:
		return fmt.Sprintf("%.1f Ki", float64(n)/float64(ki))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func overviewFrom(nodes map[string]*corev1.Node, pods map[string]*corev1.Pod, events map[string]*corev1.Event, usage cluster.Usage, usageErr error) Overview {
	o := Overview{
		CPU:         metricsCardText(usageErr, usage.Available, formatCPU(usage.CPUMilli)),
		Memory:      metricsCardText(usageErr, usage.Available, formatBytes(usage.MemoryBytes)),
		MetricsOK:   usage.Available && usageErr == nil,
		CPUMilli:    usage.CPUMilli,
		MemoryBytes: usage.MemoryBytes,
	}
	var notReady []string
	for _, n := range nodes {
		if n == nil {
			continue
		}
		o.NodesTotal++
		o.CPUAlloc += n.Status.Allocatable.Cpu().MilliValue()
		o.MemAlloc += n.Status.Allocatable.Memory().Value()
		if cluster.NodeReady(n) {
			o.NodesReady++
			continue
		}
		notReady = append(notReady, n.Name)
	}
	sort.Strings(notReady)
	if len(notReady) > 4 {
		o.NotReadyMore = len(notReady) - 4
		notReady = notReady[:4]
	}
	o.NotReady = notReady
	for _, p := range pods {
		if p == nil {
			continue
		}
		o.PodsTotal++
		switch p.Status.Phase {
		case corev1.PodRunning:
			o.PodsRunning++
		case corev1.PodPending:
			o.PodsPending++
		case corev1.PodFailed:
			o.PodsFailed++
		case corev1.PodSucceeded:
			o.PodsSucceeded++
		default:
			o.PodsUnknown++
		}
	}
	var evs []overviewEvent
	for _, e := range events {
		if e == nil || !cluster.EventWarning(e) {
			continue
		}
		o.Warnings++
		evs = append(evs, overviewEvent{
			When:    eventWhen(e),
			Object:  eventObject(e),
			Reason:  e.Reason,
			Message: trimText(e.Message, 72),
		})
	}
	sort.Slice(evs, func(i, j int) bool {
		if evs[i].When.Equal(evs[j].When) {
			return evs[i].Object < evs[j].Object
		}
		return evs[i].When.After(evs[j].When)
	})
	if len(evs) > 6 {
		evs = evs[:6]
	}
	o.Events = evs
	return o
}

func eventWhen(ev *corev1.Event) time.Time {
	switch {
	case !ev.LastTimestamp.IsZero():
		return ev.LastTimestamp.Time
	case !ev.EventTime.IsZero():
		return ev.EventTime.Time
	default:
		return ev.FirstTimestamp.Time
	}
}

func eventObject(ev *corev1.Event) string {
	ref := ev.InvolvedObject
	name := ref.Name
	if ref.Kind != "" && name != "" {
		name = ref.Kind + "/" + name
	}
	if ev.Namespace == "" {
		return name
	}
	if name == "" {
		return ev.Namespace
	}
	return ev.Namespace + "/" + name
}

func (a *App) cards(c *ui.Context, t *ui.Theme) {
	o := a.shown
	ui.Scroll(c).Grow(1).FillHeight().Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Text(c, "Overview").FontSize(22).Bold()
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				a.statCard(c, t, "Nodes", o.NodesText(), "nodes", o.NodesTotal > 0, float64(o.NodesReady), float64(o.NodesTotal), false)
				a.statCard(c, t, "Pods", o.PodsText(), "pods", o.PodsTotal > 0, float64(o.PodsRunning), float64(o.PodsTotal), false)
				a.statCard(c, t, "Warnings", o.WarningsText(), "events", false, 0, 1, o.Warnings > 0)
			})
			ui.Row(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				a.usageCard(c, t, "CPU", o.CPU, o.cpuScale, o.CPUAlloc > 0 && o.MetricsOK, formatCPU(o.CPUAlloc))
				a.usageCard(c, t, "Memory", o.Memory, o.memScale, o.MemAlloc > 0 && o.MetricsOK, formatBytes(o.MemAlloc))
			})
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				a.phasePanel(c, t, o)
				a.nodePanel(c, t, o)
			})
			a.warningPanel(c, t, o)
		})
	})
}

func (a *App) statCard(c *ui.Context, t *ui.Theme, title, value, dest string, meter bool, used, total float64, danger bool) {
	card := ui.Column(c).Grow(1).MinWidth(0).Padding(16).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Tooltip("Open " + title)
	card.Children(func() {
		ui.Text(c, title).TextColor(t.TextMuted)
		num := ui.Text(c, value).FontSize(28).Bold()
		if danger {
			num.TextColor(t.Danger)
		}
		if meter {
			var levels *ui.MeterLevels
			if used < total {
				levels = &ui.MeterLevels{Warning: 0.01, Critical: total + 1}
			}
			ui.Meter(c, used, 0, total, levels).Label(title)
		}
	})
	if card.Clicked() {
		a.openPage(dest)
	}
}

func (a *App) usageCard(c *ui.Context, t *ui.Theme, title, value string, scale func() (float64, float64, bool), showOf bool, of string) {
	ui.Column(c).Grow(1).MinWidth(0).Padding(16).Gap(8).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, title).TextColor(t.TextMuted)
		size := float32(28)
		if value == "metrics unavailable" {
			size = 16
		}
		ui.Text(c, value).FontSize(size).Bold()
		used, hi, ok := scale()
		if ok {
			ui.Meter(c, used, 0, hi, &ui.MeterLevels{Warning: hi * 0.8, Critical: hi * 0.95}).Label(title)
			if showOf {
				ui.Text(c, "of "+of).TextColor(t.TextMuted)
			}
		}
	})
}

func (a *App) phasePanel(c *ui.Context, t *ui.Theme, o Overview) {
	ui.Column(c).Grow(1).MinWidth(0).Padding(16).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "Pod phases").FontSize(16).Bold()
		if o.PodsTotal == 0 {
			ui.Text(c, "No pods").TextColor(t.TextMuted)
			return
		}
		for _, phase := range o.phases() {
			name, count, color := phase.Name, phase.Count, phase.Color(t)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(8).Height(8).Radius(4).Shrink(0).Background(color)
				ui.Text(c, name).Width(84)
				ui.Text(c, fmt.Sprintf("%d", count)).Width(28)
				track := ui.Box(c).Height(8).Radius(4).Grow(1).MinWidth(0).Background(t.Border).Clip()
				if count > 0 {
					track.Children(func() {
						ui.Box(c).Height(8).WidthPercent(float32(count) / float32(o.PodsTotal) * 100).Background(color)
					})
				}
			})
		}
	})
}

func (a *App) nodePanel(c *ui.Context, t *ui.Theme, o Overview) {
	ui.Column(c).Grow(1).MinWidth(0).Padding(16).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "Node health").FontSize(16).Bold()
		if o.NodesTotal == 0 {
			ui.Text(c, "No nodes").TextColor(t.TextMuted)
			return
		}
		ui.Text(c, fmt.Sprintf("%d of %d ready", o.NodesReady, o.NodesTotal)).TextColor(t.TextMuted)
		var levels *ui.MeterLevels
		if o.NodesReady < o.NodesTotal {
			levels = &ui.MeterLevels{Warning: 0.01, Critical: float64(o.NodesTotal) + 1}
		}
		ui.Meter(c, float64(o.NodesReady), 0, float64(o.NodesTotal), levels).Label("Ready nodes")
		if len(o.NotReady) == 0 {
			ui.Text(c, "All reported nodes are ready").TextColor(t.TextMuted)
			return
		}
		ui.Text(c, "Not ready").TextColor(t.Danger)
		for _, name := range o.NotReady {
			ui.Text(c, name)
		}
		if o.NotReadyMore > 0 {
			ui.Text(c, fmt.Sprintf("+%d", o.NotReadyMore)).TextColor(t.TextMuted)
		}
	})
}

func (a *App) warningPanel(c *ui.Context, t *ui.Theme, o Overview) {
	ui.Column(c).Padding(16).Gap(10).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "Latest warnings").FontSize(16).Bold()
		if len(o.Events) == 0 {
			ui.Text(c, "No warnings").TextColor(t.TextMuted)
			return
		}
		now := time.Now()
		for _, ev := range o.Events {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, ev.Reason).Bold().Width(120)
				ui.Text(c, ev.Object).Grow(1).MinWidth(0)
				if age := shortAge(ev.When, now); age != "" {
					ui.Text(c, age).TextColor(t.TextMuted).Shrink(0)
				}
			})
			if ev.Message != "" {
				ui.Text(c, ev.Message).TextColor(t.TextMuted)
			}
		}
	})
}

func shortAge(created, now time.Time) string {
	if created.IsZero() {
		return ""
	}
	d := now.Sub(created)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
