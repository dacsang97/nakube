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
