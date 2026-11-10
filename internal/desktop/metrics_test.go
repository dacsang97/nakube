package desktop

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/metrics"
)

func metricsApp(fetch metricsFetcher) *App {
	a := New(cluster.NewSession())
	a.redraw = func() {}
	a.metricsFetch = fetch
	return a
}

func ready(name string) metrics.View {
	return metrics.View{Source: "monitoring/prometheus-operated:9090", Results: []metrics.Result{{
		Name:   name,
		Points: []cluster.PromPoint{{At: time.Unix(0, 0), Value: 1, OK: true}},
	}}}
}

// settle waits for the fetches the app started to land.
func settle(t *testing.T, a *App, key metricsKey) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		e := a.metricsCache[key]
		done := e != nil && !e.busy
		a.mu.Unlock()
		if done {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("fetch did not land")
}

func TestMetricsFetchOnceThenWhenDue(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	a := metricsApp(func(_ context.Context, _ metricsKey) metrics.View {
		calls.Add(1)
		<-release
		return ready("Usage")
	})
	key := metricsKey{Namespace: "shop", Pod: "web", Tab: cluster.MetricCPU, Range: "1h"}
	now := time.Now()
	if v := a.metricsView(key, now); !v.Loading {
		t.Fatal("first draw must load")
	}
	a.metricsView(key, now)
	close(release)
	settle(t, a, key)
	if calls.Load() != 1 {
		t.Fatalf("fetched %d times while one was in flight", calls.Load())
	}
	v := a.metricsView(key, time.Now())
	if v.Loading || len(v.Results) != 1 || v.Tab != cluster.MetricCPU {
		t.Fatalf("fetched view %+v", v)
	}
	if calls.Load() != 1 {
		t.Fatal("fresh chart fetched again")
	}
	a.metricsView(key, time.Now().Add(metricsEvery))
	settle(t, a, key)
	if calls.Load() != 2 {
		t.Fatalf("due chart was not fetched, calls %d", calls.Load())
	}
}

func TestMetricsKeepsChartWhileRangeLoads(t *testing.T) {
	release := make(chan struct{})
	a := metricsApp(func(_ context.Context, key metricsKey) metrics.View {
		if key.Range != "1h" {
			<-release
		}
		return ready("Usage")
	})
	defer close(release)
	hour := metricsKey{Namespace: "shop", Pod: "web", Tab: cluster.MetricCPU, Range: "1h"}
	a.metricsView(hour, time.Now())
	settle(t, a, hour)
	a.metricsView(hour, time.Now())

	six := hour
	six.Range = "6h"
	v := a.metricsView(six, time.Now())
	if !v.Loading || len(v.Results) != 1 {
		t.Fatalf("new range dropped the chart: %+v", v)
	}
	other := six
	other.Pod = "api"
	if v := a.metricsView(other, time.Now()); len(v.Results) != 0 {
		t.Fatal("another pod showed this pod's chart")
	}
	tab := six
	tab.Tab = cluster.MetricMemory
	if v := a.metricsView(tab, time.Now()); len(v.Results) != 0 {
		t.Fatal("another tab showed CPU values")
	}
}

func TestMetricsEvictsAndForgets(t *testing.T) {
	a := metricsApp(func(context.Context, metricsKey) metrics.View { return ready("Usage") })
	old := metricsKey{Pod: "old", Tab: cluster.MetricCPU, Range: "1h"}
	a.metricsView(old, time.Now())
	settle(t, a, old)
	fresh := metricsKey{Pod: "new", Tab: cluster.MetricCPU, Range: "1h"}
	a.metricsView(fresh, time.Now().Add(metricsKeep+time.Second))
	a.mu.Lock()
	_, kept := a.metricsCache[old]
	a.mu.Unlock()
	if kept {
		t.Fatal("unseen chart was kept")
	}
	settle(t, a, fresh)
	a.mu.Lock()
	a.forgetMetricsLocked()
	n := len(a.metricsCache)
	a.mu.Unlock()
	if n != 0 {
		t.Fatal("forget kept charts")
	}
}

func TestMetricsStaleFetchLandsNowhere(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	a := metricsApp(func(context.Context, metricsKey) metrics.View {
		if calls.Add(1) == 1 {
			<-release
			return ready("Old")
		}
		return ready("New")
	})
	key := metricsKey{Pod: "web", Tab: cluster.MetricCPU, Range: "1h"}
	a.metricsView(key, time.Now())
	a.mu.Lock()
	a.forgetMetricsLocked()
	a.mu.Unlock()
	a.metricsView(key, time.Now())
	settle(t, a, key)
	close(release)
	time.Sleep(50 * time.Millisecond)
	v := a.metricsView(key, time.Now())
	if len(v.Results) != 1 || v.Results[0].Name != "New" {
		t.Fatalf("the forgotten fetch replaced the chart: %+v", v)
	}
}

func TestPromOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nakube", "prometheus.json")
	if err := writePromOverrides(path, map[string]string{"prod": "monitoring/prometheus-operated:9090"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %v, %v", info, err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePromOverrides(path, map[string]string{"prod": "monitoring/prometheus-operated:9090"}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("rewrite kept mode %v", info.Mode().Perm())
	}
	if got := readPromOverrides(path); got["prod"] != "monitoring/prometheus-operated:9090" {
		t.Fatalf("round trip %v", got)
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"a":"../../etc:1","b":"obs/thanos:10902"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readPromOverrides(bad); len(got) != 1 || got["b"] != "obs/thanos:10902" {
		t.Fatalf("invalid entry kept %v", got)
	}
	if got := readPromOverrides(filepath.Join(t.TempDir(), "absent")); len(got) != 0 {
		t.Fatal("absent file gave overrides")
	}
}

func TestSavePromOverrideRejectsBadDraft(t *testing.T) {
	a := metricsApp(nil)
	if msg := a.savePromOverride("prod", "prometheus:9090"); msg == "" {
		t.Fatal("a draft without a namespace was saved")
	}
}
