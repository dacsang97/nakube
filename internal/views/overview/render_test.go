package overview

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"

	"github.com/dacsang97/nakube/internal/cluster"
)

func TestRenderFrames(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	nodes := map[string]*corev1.Node{
		"cp": node("cp", true, true, "2000m", "4Gi", "20"),
		"wk": node("wk", true, false, "8000m", "16Gi", "110"),
	}
	pods := map[string]*corev1.Pod{
		"api": pod("api", "wk", corev1.PodRunning, "500m", "1000m", "256Mi", "512Mi"),
		"web": pod("web", "wk", corev1.PodRunning, "200m", "400m", "128Mi", "256Mi"),
		"sys": pod("sys", "cp", corev1.PodPending, "100m", "200m", "64Mi", "128Mi"),
	}
	events := map[string]*corev1.Event{
		"new": warning("ns", "Pod", "api", "Failed", "secret-token was not found", now.Add(-2*time.Minute)),
	}
	snap := From(nodes, pods, events, cluster.Usage{Available: true, CPUMilli: 900, MemoryBytes: 2 * 1024 * 1024 * 1024}, nil, now)
	var samples []Sample
	for i := 0; i < 12; i++ {
		samples = append(samples, Sample{
			At: now.Add(-time.Duration(12-i) * time.Minute),
			Nodes: []cluster.NodeUsage{
				{Name: "wk", CPUMilli: int64(300 + i*40), MemoryBytes: int64(400+i*20) * 1024 * 1024},
				{Name: "cp", CPUMilli: 80, MemoryBytes: 200 * 1024 * 1024},
			},
		})
	}
	role, metric, window := Worker, CPU, "15m"
	assertFrame(t, false, snap, samples, &role, &metric, &window, now)
	assertFrame(t, true, snap, samples, &role, &metric, &window, now)
	empty := From(nil, nil, nil, cluster.Usage{}, nil, now)
	assertFrame(t, false, empty, nil, &role, &metric, &window, now)
}

func assertFrame(t *testing.T, dark bool, snap Snapshot, samples []Sample, role, metric, window *string, now time.Time) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), snap, samples, "dev", role, metric, window, now, nil)
	}, 1280, 980)
	tt.SetDark(dark)
	for _, want := range []string{"Overview", "Usage", "Requests", "Limits", "Messages", "Worker", "CPU"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	if !snap.MetricsOK && !tt.HasText("metrics unavailable") {
		t.Fatal("missing metrics was drawn as a number")
	}
}
