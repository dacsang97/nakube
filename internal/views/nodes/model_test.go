package nodes

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kit "github.com/dacsang97/nakube/internal/ui"
)

func TestBuildShowsRoleVersionAndUsage(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	node := readyNode("cp", now.Add(-2*time.Hour))
	node.Labels = map[string]string{
		"node-role.kubernetes.io/control-plane": "",
		"kubernetes.io/os":                      "linux",
	}
	node.Status.NodeInfo.KubeletVersion = "v1.32.1"
	node.Status.Capacity = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}
	node.Status.Allocatable = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}
	page := Build([]*corev1.Node{node, nil}, map[string]Sample{
		"cp": {CPUMilli: 500, MemoryBytes: 1 << 30},
	}, "", All, ColName, false, now)
	if page.Total != 1 || len(page.Rows) != 1 {
		t.Fatalf("page %+v", page)
	}
	row := page.Rows[0]
	if row.Status != "Ready" || row.StatusTone != kit.ToneOK || row.Roles != "control-plane" || row.Version != "v1.32.1" {
		t.Fatalf("row %+v", row)
	}
	if !row.CPUKnown || row.CPU != 500 || row.CPUMax != 2000 || row.CPUTip == "" {
		t.Fatalf("cpu %+v tip %q", row, row.CPUTip)
	}
	if !row.MemKnown || row.Age != "2h" {
		t.Fatalf("memory known %v age %q", row.MemKnown, row.Age)
	}
	if row.Detail.Groups[0].Facts[0].Value != "2" {
		t.Fatalf("capacity %+v", row.Detail.Groups[0].Facts)
	}
}

func TestMissingMetricsStayUnknown(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	node := readyNode("wk", now)
	node.Status.Allocatable = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}
	page := Build([]*corev1.Node{node}, nil, "", All, "", false, now)
	row := page.Rows[0]
	if row.CPUKnown || row.MemKnown || row.CPU != 0 {
		t.Fatalf("missing metrics became a zero %+v", row)
	}
}

func TestNotReadyFilterAndCordon(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	bad := readyNode("bad", now)
	bad.Status.Conditions[0].Status = corev1.ConditionFalse
	cordoned := readyNode("hold", now)
	cordoned.Spec.Unschedulable = true
	page := Build([]*corev1.Node{bad, cordoned, readyNode("ok", now)}, nil, "", NotReady, ColName, false, now)
	if len(page.Rows) != 1 || page.Rows[0].Name != "bad" || page.Total != 3 {
		t.Fatalf("filter %+v", names(page))
	}
	held := Build([]*corev1.Node{cordoned}, nil, "sched", All, ColName, false, now)
	if len(held.Rows) != 1 || held.Rows[0].Conditions[0].Text != "SchedulingDisabled" {
		t.Fatalf("cordon %+v", held.Rows)
	}
}

func TestSearchMissesAndSortsByName(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	page := Build([]*corev1.Node{readyNode("b", now), readyNode("a", now)}, nil, "nope", All, ColName, false, now)
	if page.Total != 2 || len(page.Rows) != 0 {
		t.Fatalf("search %+v", page)
	}
	sorted := Build([]*corev1.Node{readyNode("b", now), readyNode("a", now)}, nil, "", All, ColName, false, now)
	if names(sorted)[0] != "a" || names(sorted)[1] != "b" {
		t.Fatalf("sort %v", names(sorted))
	}
}

func readyNode(name string, created time.Time) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created)},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
			Type:   corev1.NodeReady,
			Status: corev1.ConditionTrue,
		}}},
	}
}

func names(page Page) []string {
	out := make([]string, len(page.Rows))
	for i, row := range page.Rows {
		out[i] = row.Name
	}
	return out
}
