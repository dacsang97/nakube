package pods

import (
	"testing"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
)

func drawn(t *testing.T, page Page, onOpen func(screen, key string)) (*ui.Tester, *int) {
	t.Helper()
	query := ""
	row := -1
	state := &ui.ListState{Selected: &row}
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), page, &query, state, false, t0, onOpen, nil)
	}, 1600, 500)
	return tt, &row
}

func TestDrawColumnsAndValues(t *testing.T) {
	yes := true
	items := []*corev1.Pod{
		pod("web", func(p *corev1.Pod) {
			p.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web-1", Controller: &yes}}
		}),
		pod("done", exited(0, "Completed")),
	}
	page := Build(items, map[string]cluster.PodUsage{"shop/web": {CPUMilli: 302, MemoryBytes: 1536 * 1024 * 1024}}, "", "", false)
	tt, _ := drawn(t, page, func(string, string) {})
	for _, want := range []string{
		"Name", "Namespace", "Containers", "CPU", "Memory", "Restarts", "Controlled By", "Node", "QoS", "Age", "Status",
		"0.302", "1.5GiB", "N/A", "ReplicaSet", "node-a", "Burstable", "Completed", "Running",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
}

func TestDrawLinkOpensTarget(t *testing.T) {
	var screen, key string
	tt, _ := drawn(t, Build([]*corev1.Pod{pod("web")}, nil, "", "", false), func(s, k string) { screen, key = s, k })
	if err := tt.Click("Open node-a"); err != nil {
		t.Fatal(err)
	}
	if screen != "nodes" || key != "node-a" {
		t.Fatalf("opened %q %q", screen, key)
	}
}

func TestDrawOwnerWithoutScreenIsText(t *testing.T) {
	opened := false
	p := pod("web", func(p *corev1.Pod) {
		p.OwnerReferences = []metav1.OwnerReference{{APIVersion: "argoproj.io/v1alpha1", Kind: "Rollout", Name: "r"}}
	})
	tt, _ := drawn(t, Build([]*corev1.Pod{p}, nil, "", "", false), func(string, string) { opened = true })
	if err := tt.Click("Rollout"); err != nil {
		t.Fatal(err)
	}
	if opened {
		t.Fatal("a custom owner opened a screen")
	}
}

func TestPodLinksKeyboardAndHitArea(t *testing.T) {
	p := pod("web", func(p *corev1.Pod) {
		p.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web-1"}}
	})
	var opened []string
	tt, _ := drawn(t, Build([]*corev1.Pod{p}, nil, "", "", false), func(screen, key string) {
		opened = append(opened, screen+"/"+key)
	})
	// Search and the table are also tab stops. Enter on the table must
	// not follow a link; the first link reached is the owner.
	for range 8 {
		tt.Key(0, ui.KeyTab)
		tt.Key(0, ui.KeyEnter)
		if len(opened) > 0 {
			break
		}
	}
	if len(opened) != 1 || opened[0] != "workloads/replicasets/shop/web-1" {
		t.Fatalf("keyboard owner navigation: %v", opened)
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if len(opened) != 2 || opened[1] != "nodes/node-a" {
		t.Fatalf("keyboard node navigation: %v", opened)
	}
	for _, label := range []string{"Open ReplicaSet web-1", "Open node-a"} {
		r, ok := tt.Find(label)
		if !ok || r.W < 40 || r.H < 40 {
			t.Fatalf("%s hit area: %+v, found=%v", label, r, ok)
		}
		// Clicking near the edge, away from the text, still activates.
		tt.ClickAt(r.X+r.W-2, r.Y+r.H-2)
	}
	if len(opened) != 4 || opened[2] != opened[0] || opened[3] != opened[1] {
		t.Fatalf("expanded hit areas opened wrong targets: %v", opened)
	}
}

func TestContainerIndicatorsWrapWithoutHidingFailures(t *testing.T) {
	p := pod("web", func(p *corev1.Pod) {
		for i := range 10 {
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: string(rune('a' + i))})
		}
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{
			Name: "j", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		})
	})
	tt, _ := drawn(t, Build([]*corev1.Pod{p}, nil, "", "", false), nil)
	for _, width := range []int{1600, 800} {
		tt.SetSize(width, 500)
		first, ok := tt.Find("app: running, ready")
		if !ok {
			t.Fatal("missing first container")
		}
		last, ok := tt.Find("j: waiting, CrashLoopBackOff")
		if !ok || last.Y <= first.Y || last.X < first.X || last.X+last.W > first.X+76 || last.Y+last.H > 500 {
			t.Fatalf("failure indicator hidden at width %d: first=%+v last=%+v found=%v", width, first, last, ok)
		}
	}
}
