package metrics

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dacsang97/nakube/internal/cluster"
)

func TestFormats(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{CPU(0), "0"},
		{CPU(0.0015), "1.5m"},
		{CPU(0.35), "350m"},
		{CPU(1.234), "1.23"},
		{Bytes(0), "0"},
		{Bytes(512), "512B"},
		{Bytes(1536), "1.5Ki"},
		{Bytes(256 * 1024 * 1024), "256Mi"},
		{Format(cluster.MetricNetwork)(2048), "2Ki/s"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestRangeDur(t *testing.T) {
	if RangeDur("7d") != 7*24*time.Hour || RangeDur("bogus") != time.Hour {
		t.Fatal("range")
	}
}

func points(ok ...bool) []cluster.PromPoint {
	out := make([]cluster.PromPoint, len(ok))
	for i, v := range ok {
		out[i] = cluster.PromPoint{At: time.Unix(int64(i*60), 0), Value: 1, OK: v}
	}
	return out
}

func TestTracesDropEmptySeries(t *testing.T) {
	got := Traces(cluster.MetricCPU, []Result{
		{Name: "Usage", Points: points(true, false)},
		{Name: "Requests", Line: true, Points: points(false, false)},
		{Name: "Limits", Line: true, Points: points(true, true)},
	})
	if len(got) != 2 || got[0].Name != "Usage" || got[1].Name != "Limits" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Points[1].OK {
		t.Fatal("gap became a value")
	}
	if got[1].Color == got[0].Color {
		t.Fatal("limits kept the bar color")
	}
}

func TestTracesPromoteLineWithoutBars(t *testing.T) {
	got := Traces(cluster.MetricFilesystem, []Result{
		{Name: "Usage", Points: points(false)},
		{Name: "Reads", Line: true, Points: points(true)},
		{Name: "Writes", Line: true, Points: points(true)},
	})
	if len(got) != 2 || got[0].Name != "Reads" || got[0].Line || !got[1].Line {
		t.Fatalf("got %+v", got)
	}
	if got[0].Color == got[1].Color {
		t.Fatal("promoted bars kept the line color of the other series")
	}
}

func TestTracesNeverDrawRequestsAsUsage(t *testing.T) {
	got := Traces(cluster.MetricCPU, []Result{
		{Name: "Usage", Points: points(false)},
		{Name: "Requests", Line: true, Points: points(true)},
	})
	if len(got) != 1 || !got[0].Line {
		t.Fatalf("requests became bars: %+v", got)
	}
}

func TestMessage(t *testing.T) {
	if Message(nil) != "" {
		t.Fatal("nil error drew a message")
	}
	if !strings.Contains(Message(fmt.Errorf("x: %w", cluster.ErrPromNotFound)), "Settings") {
		t.Fatal("not found must point at Settings")
	}
	forbidden := apierrors.NewForbidden(schema.GroupResource{Resource: "services"}, "p", errors.New("no"))
	if !strings.Contains(Message(forbidden), "services/proxy") {
		t.Fatal("forbidden must name the permission")
	}
	if !strings.Contains(Message(fmt.Errorf("%w: %w", cluster.ErrPromListDenied, forbidden)), "Cannot list services") {
		t.Fatal("a denied list must not ask for services/proxy")
	}
	gone := apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, "p")
	if !SourceGone(gone) || !strings.Contains(Message(gone), "Settings") {
		t.Fatal("a missing service must point at Settings")
	}
}

func TestDrawStates(t *testing.T) {
	draw := func(v View) *ui.Tester {
		tab, rng, hover := v.Tab, DefaultRange, -1
		return ui.NewTester(func(c *ui.Context) {
			Draw(c, c.Theme(), v, &tab, &rng, &hover, Icons{})
		}, 520, 400)
	}
	if tt := draw(View{Tab: cluster.MetricCPU, Loading: true}); !tt.HasText("Metrics") {
		t.Fatalf("loading lost the title %v", tt.Texts())
	}
	if tt := draw(View{Tab: cluster.MetricCPU, Err: "Prometheus not found. Set its service in Settings."}); !tt.HasText("Prometheus not found. Set its service in Settings.") {
		t.Fatalf("error not shown %v", tt.Texts())
	}
	tt := draw(View{Tab: cluster.MetricCPU, Source: "monitoring/prometheus-operated:9090", Results: []Result{{Name: "Usage", Points: points(true, true)}}})
	if !tt.HasText("Displaying metrics from Prometheus: monitoring/prometheus-operated:9090") || !tt.HasText("Usage") {
		t.Fatalf("ready view %v", tt.Texts())
	}
	if tt := draw(View{Tab: cluster.MetricCPU, Results: []Result{{Name: "Usage", Points: points(false)}}}); !tt.HasText("No data in this range") {
		t.Fatalf("empty view %v", tt.Texts())
	}
}

func TestMetricTabsKeyboardAndNarrowHitAreas(t *testing.T) {
	tab, rng, hover := DefaultTab, DefaultRange, 12
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), View{Tab: tab}, &tab, &rng, &hover, Icons{})
	}, 320, 500)
	for _, label := range []string{"CPU", "Memory", "Network", "Filesystem", "Range"} {
		r, ok := tt.Find(label)
		if !ok || r.W < 40 || r.H < 40 || r.X < 0 || r.X+r.W > 320 {
			t.Fatalf("%s hit area is clipped or too small: %+v, found=%v", label, r, ok)
		}
	}
	if err := tt.Click("Memory"); err != nil {
		t.Fatal(err)
	}
	if tab != cluster.MetricMemory || hover != -1 {
		t.Fatalf("click: tab=%s hover=%d", tab, hover)
	}
	tt.Key(0, ui.KeyRight)
	if tab != cluster.MetricNetwork {
		t.Fatalf("right arrow selected %s", tab)
	}
	tt.Key(0, ui.KeyEnd)
	if tab != cluster.MetricFilesystem {
		t.Fatalf("End selected %s", tab)
	}
	tt.Key(0, ui.KeyHome)
	if tab != cluster.MetricCPU {
		t.Fatalf("Home selected %s", tab)
	}
}

func TestFilesystemSeparatesCapacityAndRates(t *testing.T) {
	tab, rng, hover := cluster.MetricFilesystem, DefaultRange, 0
	v := View{Tab: tab, Results: []Result{
		{Name: "Usage", Points: []cluster.PromPoint{{At: time.Unix(0, 0), Value: 1 << 30, OK: true}}},
		{Name: "Reads", Line: true, Points: []cluster.PromPoint{{At: time.Unix(0, 0), Value: 2048, OK: true}}},
		{Name: "Writes", Line: true, Points: []cluster.PromPoint{{At: time.Unix(0, 0), Value: 4096, OK: true}}},
	}}
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), v, &tab, &rng, &hover, Icons{})
	}, 520, 700)
	for _, text := range []string{"Usage (bytes)", "Throughput (bytes/s)", "Usage  1Gi", "Reads  2Ki/s", "Writes  4Ki/s"} {
		if !tt.HasText(text) {
			t.Fatalf("missing dimension-correct display %q: %v", text, tt.Texts())
		}
	}
}
