package ui

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func traces() []Trace {
	start := time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	usage := make([]Sample, 10)
	limit := make([]Sample, 10)
	for i := range usage {
		at := start.Add(time.Duration(i) * time.Minute)
		usage[i] = Sample{At: at, Value: float64(i + 1), OK: i != 4}
		limit[i] = Sample{At: at, Value: 20, OK: true}
	}
	return []Trace{
		{Name: "Usage", Color: Colors().Usage, Points: usage},
		{Name: "Limits", Color: Colors().Limits, Points: limit, Line: true},
	}
}

func TestMetricsHigh(t *testing.T) {
	if MetricsHigh(nil) != 1 {
		t.Fatal("no data must keep a unit axis")
	}
	if got := MetricsHigh(traces()); got != 22 {
		t.Fatalf("high = %v, want limits with headroom", got)
	}
}

func TestMetricsChartHover(t *testing.T) {
	hover := 4
	format := func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) }
	tt := ui.NewTester(func(c *ui.Context) {
		MetricsChart(c, c.Theme(), traces(), format, &hover)
		MetricsLegend(c, c.Theme(), traces())
	}, 600, 300)
	if !tt.HasText("14:00") {
		t.Fatalf("no time axis %v", tt.Texts())
	}
	if !tt.HasText("Usage  —") {
		t.Fatalf("gap slot must read as a dash, got %v", tt.Texts())
	}
	if !tt.HasText("Limits  20") || !tt.HasText("14:04") {
		t.Fatalf("tooltip lost a series %v", tt.Texts())
	}
}

func TestMetricsChartNoHover(t *testing.T) {
	hover := -1
	tt := ui.NewTester(func(c *ui.Context) {
		MetricsChart(c, c.Theme(), traces(), nil, &hover)
	}, 600, 300)
	if tt.HasText("Limits  20") {
		t.Fatal("tooltip drawn without a hover")
	}
}

func TestMetricsChartEmpty(t *testing.T) {
	hover := -1
	tt := ui.NewTester(func(c *ui.Context) {
		MetricsChart(c, c.Theme(), nil, nil, &hover)
	}, 600, 300)
	if len(tt.Texts()) != 0 {
		t.Fatalf("empty chart drew %v", tt.Texts())
	}
}
