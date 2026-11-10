// Package metrics is a resource's Metrics block: a range, a tab per
// resource, and a chart of Prometheus series.
package metrics

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
)

// DefaultRange and DefaultTab are where a newly chosen resource starts.
const (
	DefaultRange = "1h"
	DefaultTab   = cluster.MetricCPU
)

// Ranges are the choices of the range menu.
func Ranges() []string {
	return []string{"1h", "6h", "24h", "7d"}
}

// RangeDur is the span of a range label. An unknown label is an hour.
func RangeDur(label string) time.Duration {
	switch label {
	case "6h":
		return 6 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// TabLabel names a tab for its tooltip.
func TabLabel(tab string) string {
	switch tab {
	case cluster.MetricCPU:
		return "CPU"
	case cluster.MetricMemory:
		return "Memory"
	case cluster.MetricNetwork:
		return "Network"
	case cluster.MetricFilesystem:
		return "Filesystem"
	default:
		return tab
	}
}

// Result is one series a query returned, before it has a color.
type Result struct {
	Name   string
	Line   bool
	Points []cluster.PromPoint
}

// View is what the block shows. Source is "ns/svc:port" once found. Err is
// a sentence for the reader; it wins over stale results.
type View struct {
	Source  string
	Tab     string
	Results []Result
	Loading bool
	Err     string
}

// palette colors series by position, so the bar is always the same blue
// and the lines over it stay apart from it and from the cursor.
func palette() []ui.Color {
	s := kit.Colors()
	return []ui.Color{s.CPU, s.Requests, s.Usage}
}

// Traces gives each result its color. A series with no real point is left
// out, so a cluster without kube-state-metrics draws no request line.
func Traces(tab string, results []Result) []kit.Trace {
	colors := palette()
	out := make([]kit.Trace, 0, len(results))
	for i, r := range results {
		if !anyOK(r.Points) {
			continue
		}
		samples := make([]kit.Sample, len(r.Points))
		for j, p := range r.Points {
			samples[j] = kit.Sample{At: p.At, Value: p.Value, OK: p.OK}
		}
		out = append(out, kit.Trace{Name: r.Name, Color: colors[i%len(colors)], Points: samples, Line: r.Line})
	}
	// containerd's kubelet reports no container_fs_usage_bytes, so the
	// filesystem tab can lose its bars. The first series left takes them.
	// Other tabs keep their lines: a request is never drawn as usage.
	if tab != cluster.MetricFilesystem {
		return out
	}
	for _, tr := range out {
		if !tr.Line {
			return out
		}
	}
	if len(out) > 0 {
		out[0].Line = false
		out[0].Color = colors[0]
	}
	return out
}

func anyOK(points []cluster.PromPoint) bool {
	for _, p := range points {
		if p.OK {
			return true
		}
	}
	return false
}

// Format is the value format of a tab's axis and tooltip.
func Format(tab string) func(float64) string {
	switch tab {
	case cluster.MetricCPU:
		return CPU
	case cluster.MetricNetwork:
		return func(v float64) string { return Bytes(v) + "/s" }
	default:
		return Bytes
	}
}

// CPU is cores as Kubernetes writes them: millicores under one core.
func CPU(cores float64) string {
	if cores <= 0 {
		return "0"
	}
	if cores < 1 {
		m := cores * 1000
		if m < 10 {
			return trim(m, 1) + "m"
		}
		return strconv.Itoa(int(math.Round(m))) + "m"
	}
	return trim(cores, 2)
}

// Bytes is a binary size: Ki, Mi, Gi, Ti.
func Bytes(v float64) string {
	if v <= 0 {
		return "0"
	}
	units := []string{"B", "Ki", "Mi", "Gi", "Ti"}
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0fB", v)
	}
	return trim(v, 1) + units[i]
}

func trim(v float64, decimals int) string {
	p := math.Pow10(decimals)
	return strconv.FormatFloat(math.Round(v*p)/p, 'f', -1, 64)
}

// SourceGone means the proxied service is missing or has no ready pods.
func SourceGone(err error) bool {
	return apierrors.IsNotFound(err) || apierrors.IsServiceUnavailable(err)
}

// Message turns a fetch error into a sentence. It never shows a token or
// a URL with credentials, only what the reader can act on.
func Message(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, cluster.ErrPromNotFound):
		return "Prometheus not found. Set its service in Settings."
	case errors.Is(err, cluster.ErrPromListDenied):
		return "Cannot list services to find Prometheus. Set its service in Settings."
	case SourceGone(err):
		return "Prometheus service not found or not ready. Check it in Settings."
	case cluster.PromForbidden(err):
		return "Needs permission to proxy services (services/proxy)."
	default:
		return "Metrics query failed: " + err.Error()
	}
}
