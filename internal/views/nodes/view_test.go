package nodes

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kit "github.com/dacsang97/nakube/internal/ui"
)

func TestDrawShowsStatusRoleAndUnknownUsage(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	node := readyNode("cp", now)
	node.Labels = map[string]string{"node-role.kubernetes.io/control-plane": ""}
	node.Status.Allocatable = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}
	page := Build([]*corev1.Node{node}, nil, "", All, ColName, false, now)
	query := ""
	filter := All
	selected := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), page, &query, &filter, &ui.ListState{Selected: &selected}, false)
	}, 1100, 700)
	for _, want := range []string{"Nodes", "cp", "Ready", "control-plane", "CPU", "—"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %v", want, tt.Texts())
		}
	}
}

func TestRowClickChangesSelection(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	page := Build([]*corev1.Node{readyNode("cp", now)}, nil, "", All, ColName, false, now)
	query := ""
	filter := All
	selected := -1
	state := &ui.ListState{Selected: &selected}
	var selection kit.Selection
	tt := ui.NewTester(func(c *ui.Context) {
		selection = Draw(c, c.Theme(), page, &query, &filter, state, false)
	}, 1100, 700)
	if err := tt.Click("cp"); err != nil {
		t.Fatal(err)
	}
	if !selection.Valid || selected != 0 {
		t.Fatalf("row selection not reported: %+v selected %d", selection, selected)
	}
}

func TestDrawLoadingSkipsEmpty(t *testing.T) {
	query := ""
	filter := All
	tt := ui.NewTester(func(c *ui.Context) {
		Draw(c, c.Theme(), Page{}, &query, &filter, &ui.ListState{}, true)
	}, 800, 600)
	if tt.HasText("None") || !tt.HasText("Nodes") || !tt.HasText("Status") {
		t.Fatalf("loading %v", tt.Texts())
	}
}
