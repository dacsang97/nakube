package desktop

import (
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/overview"
)

func TestPhoneOverviewDoesNotInventUtilization(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Status: corev1.NodeStatus{
		Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("4Gi")},
		Conditions:  []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
	}}
	nodes := map[string]*corev1.Node{"worker": node}
	for _, test := range []struct {
		name      string
		usage     cluster.Usage
		err       error
		available bool
	}{
		{name: "missing metrics"},
		{name: "metrics forbidden", usage: cluster.Usage{Available: true, Nodes: []cluster.NodeUsage{{Name: "worker"}}}, err: errors.New("forbidden")},
		{name: "partial response", usage: cluster.Usage{Available: true}},
		{name: "wrong node", usage: cluster.Usage{Available: true, Nodes: []cluster.NodeUsage{{Name: "removed-worker"}}}},
		{name: "measured zero", usage: cluster.Usage{Available: true, Nodes: []cluster.NodeUsage{{Name: "worker"}}}, available: true},
		{name: "over capacity", usage: cluster.Usage{Available: true, CPUMilli: 2500, MemoryBytes: 1024, Nodes: []cluster.NodeUsage{{Name: "worker", CPUMilli: 2500, MemoryBytes: 1024}}}, available: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			shown := overview.From(nodes, nil, nil, test.usage, test.err, time.Now())
			body := phoneOverview("mobile", shown, test.usage)
			charts := body.Charts
			if charts.NodesReady != 1 || charts.NodesTotal != 1 || charts.CPU.Allocatable != 2000 || charts.Memory.Allocatable != 4*1024*1024*1024 {
				t.Fatalf("invalid chart capacity: %+v", charts)
			}
			if (charts.CPU.Used != nil) != test.available || (charts.Memory.Used != nil) != test.available {
				t.Fatalf("availability = cpu %v memory %v", charts.CPU.Used, charts.Memory.Used)
			}
			if test.available && (*charts.CPU.Used != test.usage.CPUMilli || *charts.Memory.Used != test.usage.MemoryBytes) {
				t.Fatal("changed observed usage")
			}
		})
	}
}
