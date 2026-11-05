package desktop

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
)

func TestOverviewCountsPhasesAndCapacity(t *testing.T) {
	nodes := map[string]*corev1.Node{
		"a": node("a", true, "2000m", "1Gi"),
		"b": node("b", false, "1000m", "1Gi"),
		"c": nil,
	}
	pods := map[string]*corev1.Pod{
		"run": {Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		"pen": {Status: corev1.PodStatus{Phase: corev1.PodPending}},
		"bad": {Status: corev1.PodStatus{Phase: corev1.PodFailed}},
		"ok":  {Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
	}
	now := time.Now()
	events := map[string]*corev1.Event{
		"old": warning("ns", "Pod", "web", "BackOff", "old", now.Add(-time.Hour)),
		"new": warning("ns", "Pod", "api", "Failed", "secret-token", now),
	}
	o := overviewFrom(nodes, pods, events, cluster.Usage{Available: true, CPUMilli: 500, MemoryBytes: 256 * 1024 * 1024}, nil)
	if o.NodesText() != "1/2" || o.PodsText() != "1/4" || o.WarningsText() != "2" {
		t.Fatalf("nodes %s pods %s warnings %s", o.NodesText(), o.PodsText(), o.WarningsText())
	}
	if o.PodsPending != 1 || o.PodsFailed != 1 || o.PodsSucceeded != 1 {
		t.Fatalf("phases %+v", o)
	}
	if !o.MetricsOK || o.CPU != "500m" || o.CPUAlloc != 3000 || o.MemAlloc != 2*1024*1024*1024 {
		t.Fatalf("usage %+v", o)
	}
	if len(o.NotReady) != 1 || o.NotReady[0] != "b" {
		t.Fatalf("not ready %v", o.NotReady)
	}
	if len(o.Events) != 2 || o.Events[0].Reason != "Failed" || o.Events[0].Object != "ns/Pod/api" {
		t.Fatalf("events %+v", o.Events)
	}
	if o.Events[0].Message != "secret-token" {
		t.Fatalf("message %q", o.Events[0].Message)
	}
	used, hi, ok := o.cpuScale()
	if !ok || used != 500 || hi != 3000 {
		t.Fatalf("scale %v %v %v", used, hi, ok)
	}
}

func TestOverviewDoesNotInventAMeter(t *testing.T) {
	o := overviewFrom(map[string]*corev1.Node{
		"a": node("a", true, "2000m", "1Gi"),
	}, nil, nil, cluster.Usage{}, nil)
	if o.MetricsOK || o.CPU != "metrics unavailable" {
		t.Fatalf("%+v", o)
	}
	if _, _, ok := o.cpuScale(); ok {
		t.Fatal("missing metrics still produced a scale")
	}
	if _, _, ok := o.memScale(); ok {
		t.Fatal("missing metrics still produced a memory scale")
	}
}

func node(name string, ready bool, cpu, mem string) *corev1.Node {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(mem),
			},
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}},
		},
	}
}

func warning(ns, kind, name, reason, message string, when time.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: ns, Name: name},
		Reason:         reason,
		Message:        message,
		Type:           corev1.EventTypeWarning,
		LastTimestamp:  metav1.NewTime(when),
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: name, Namespace: ns},
	}
}
