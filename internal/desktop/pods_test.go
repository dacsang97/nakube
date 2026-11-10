package desktop

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/metrics"
)

func podsApp(fetch podUsageFetcher) *App {
	a := New(cluster.NewSession())
	a.redraw = func() {}
	a.podUsageFetch = fetch
	a.metricsFetch = func(context.Context, metricsKey) metrics.View { return metrics.View{} }
	a.pods = map[string]*corev1.Pod{
		"shop/web": {
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "web", CreationTimestamp: metav1.Now()},
			Spec:       corev1.PodSpec{NodeName: "node-a", Containers: []corev1.Container{{Name: "app"}}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		"ops/job": {
			ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "job"},
			Spec:       corev1.PodSpec{NodeName: "node-b"},
		},
	}
	a.watchSettled = true
	a.showScreen("pods", true)
	return a
}

func noUsage(context.Context, []string) (map[string]cluster.PodUsage, bool, error) {
	return nil, false, nil
}

func TestPodsScreenIsInspect(t *testing.T) {
	if !inspectScreen("pods") {
		t.Fatal("pods still draws the generic table")
	}
}

func TestDrawPodsShowsUsageAndFilter(t *testing.T) {
	a := podsApp(func(context.Context, []string) (map[string]cluster.PodUsage, bool, error) {
		return map[string]cluster.PodUsage{"shop/web": {CPUMilli: 302, MemoryBytes: 1 << 30}}, true, nil
	})
	tt := ui.NewTester(func(c *ui.Context) { a.drawPods(c, c.Theme()) }, 1600, 500)
	deadline := time.Now().Add(2 * time.Second)
	for !tt.HasText("0.302") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("0.302") || !tt.HasText("1GiB") || !tt.HasText("N/A") {
		t.Fatalf("usage not drawn %v", tt.Texts())
	}
	a.nsFilter = "ops"
	tt.Frame()
	if tt.HasText("web") || !tt.HasText("job") {
		t.Fatalf("namespace filter ignored %v", tt.Texts())
	}
}

func TestPodLinkOpensNodeWithRowChosen(t *testing.T) {
	a := podsApp(noUsage)
	a.listQuery = "web"
	tt := ui.NewTester(func(c *ui.Context) {
		if a.tabs.Screen(a.active) == "pods" {
			a.drawPods(c, c.Theme())
		}
	}, 1600, 500)
	if err := tt.Click("Open node-a"); err != nil {
		t.Fatal(err)
	}
	if got := a.tabs.Screen(a.active); got != "nodes" {
		t.Fatalf("active screen %q", got)
	}
	if a.listKey != "node-a" || a.row != -1 || a.listQuery != "" || !a.listReveal || a.detailPanel != nil {
		t.Fatalf("nodes tab key %q row %d query %q reveal %v", a.listKey, a.row, a.listQuery, a.listReveal)
	}
	pods, _ := a.tabs.byScreen("pods")
	if b := a.bodies[pods]; b == nil || b.listKey == "node-a" || b.listQuery != "web" {
		t.Fatalf("pods tab state was overwritten: %+v", b)
	}
}

func TestOpenWorkloadDropsHidingNamespace(t *testing.T) {
	a := podsApp(noUsage)
	a.showScreen("workloads", false)
	a.nsFilter = "ops"
	a.showScreen("pods", true)
	a.openListAt("workloads", "replicasets/shop/web-1")
	if a.tabs.Screen(a.active) != "workloads" || a.nsFilter != "" || a.listKey != "replicasets/shop/web-1" {
		t.Fatalf("screen %q ns %q key %q", a.tabs.Screen(a.active), a.nsFilter, a.listKey)
	}
}

func TestBindListRevealsLinkedRow(t *testing.T) {
	a := podsApp(noUsage)
	a.listKey, a.row, a.listReveal, a.detailPanelOpen = "b", -1, true, false
	a.bindList([]string{"a", "b"})
	if a.row != 1 || a.listReveal || !a.detailPanelOpen {
		t.Fatalf("row %d reveal %v panel %v", a.row, a.listReveal, a.detailPanelOpen)
	}
}

func TestPodUsageFetchesOnceAndDropsStale(t *testing.T) {
	release := make(chan struct{})
	done := make(chan struct{})
	var calls atomic.Int32
	a := podsApp(func(context.Context, []string) (map[string]cluster.PodUsage, bool, error) {
		calls.Add(1)
		<-release
		defer close(done)
		return map[string]cluster.PodUsage{"shop/web": {CPUMilli: 1}}, true, nil
	})
	a.mu.Lock()
	a.ensurePodUsageLocked(time.Now(), nil)
	a.ensurePodUsageLocked(time.Now(), nil)
	a.forgetPodUsageLocked()
	a.mu.Unlock()
	close(release)
	<-done
	// The write-back follows under the lock the fetch takes next.
	time.Sleep(10 * time.Millisecond)
	a.mu.Lock()
	defer a.mu.Unlock()
	if calls.Load() != 1 {
		t.Fatalf("fetched %d times", calls.Load())
	}
	if a.podUsage != nil {
		t.Fatal("a read from before the switch landed")
	}
}

func TestReadyDots(t *testing.T) {
	if got := readyDots(nil); got != "0/0" {
		t.Fatalf("got %s", got)
	}
}
