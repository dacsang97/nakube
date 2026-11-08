package workloads

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRenderWorkloadOverview(t *testing.T) {
	one := int32(1)
	page := Build([]runtime.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod"}, Spec: appsv1.DeploymentSpec{Replicas: &one}, Status: appsv1.DeploymentStatus{ReadyReplicas: 0}},
	}, "", 0, ColKind, false, "")
	query := ""
	filter := 0

	for _, width := range []int{720, 1280} {
		selected := -1
		sort := ui.SortOrder{Column: ColKind}
		state := ui.ListState{Selected: &selected, Sort: &sort}
		tt := ui.NewTester(func(c *ui.Context) {
			Draw(c, c.Theme(), page, "dev", &query, &filter, &state, false, time.Now(), nil)
		}, width, 900)
		for _, want := range []string{"Workloads", "Overall readiness", "Health by kind", "Inventory", "Pod", "Deployment", "Healthy", "Active"} {
			if !tt.HasText(want) {
				t.Errorf("width %d missing %q in %v", width, want, tt.Texts())
			}
		}
	}
}

func TestRowClickRendersDetailPanelContent(t *testing.T) {
	one := int32(1)
	page := Build([]runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "prod", CreationTimestamp: metav1.NewTime(time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC))},
			Spec:       appsv1.DeploymentSpec{Replicas: &one},
			Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
		},
	}, "", 0, ColKind, false, "")
	query := ""
	filter := 0
	selected := -1
	sort := ui.SortOrder{Column: ColKind}
	state := ui.ListState{Selected: &selected, Sort: &sort}
	open := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Gap(4).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				selection := Draw(c, c.Theme(), page, "dev", &query, &filter, &state, false, time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC), nil)
				if selection.Changed {
					open = true
				}
				if open && selection.Valid {
					ui.Column(c).Width(360).FillHeight().Children(func() {
						Detail(c, c.Theme(), selection.Row, time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
					})
				}
			})
		})
	}, 900, 700)

	if err := tt.Click("web"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Overview", "Capacity", "Created", "Nov 6, 2026 10:00"} {
		if !tt.HasText(want) {
			t.Fatalf("detail panel body missing %q in %v", want, tt.Texts())
		}
	}
}

func TestRenderWorkloadLoadingKeepsChrome(t *testing.T) {
	query := ""
	filter := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), Page{Kinds: kindSummaries()}, "dev", &query, &filter, &ui.ListState{}, true, time.Now(), nil)
	}, 900, 700)
	for _, want := range []string{"Workloads", "Total", "Overall readiness", "Health by kind", "Inventory"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
	if tt.HasText("0 resources across 0 kinds") {
		t.Fatal("loading screen presented settled zeroes")
	}
}
