package overview

import (
	"math"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
)

func TestFromCountsPhasesAndKeepsPhoneText(t *testing.T) {
	nodes := map[string]*corev1.Node{
		"a": node("a", true, false, "2000m", "1Gi", "10"),
		"b": node("b", false, true, "1000m", "1Gi", "8"),
		"c": nil,
	}
	pods := map[string]*corev1.Pod{
		"run": pod("run", "a", corev1.PodRunning, "100m", "200m", "64Mi", "128Mi"),
		"pen": pod("pen", "b", corev1.PodPending, "50m", "50m", "32Mi", "32Mi"),
		"bad": pod("bad", "", corev1.PodFailed, "10m", "10m", "1Mi", "1Mi"),
		"ok":  pod("ok", "a", corev1.PodSucceeded, "0", "0", "0", "0"),
	}
	now := time.Now()
	events := map[string]*corev1.Event{
		"old": warning("ns", "Pod", "web", "BackOff", "old", now.Add(-time.Hour)),
		"new": warning("ns", "Pod", "api", "Failed", "secret-token", now),
	}
	s := From(nodes, pods, events, cluster.Usage{Available: true, CPUMilli: 500, MemoryBytes: 256 * 1024 * 1024}, nil, now)
	if s.NodesText() != "1/2" || s.PodsText() != "1/4" || s.WarningsText() != "2" {
		t.Fatalf("nodes %s pods %s warnings %s", s.NodesText(), s.PodsText(), s.WarningsText())
	}
	if s.CPU != "500m" || s.Memory != "256.0 Mi" || !s.MetricsOK {
		t.Fatalf("phone text %+v", s)
	}
	if len(s.NotReady) != 1 || s.NotReady[0] != "b" {
		t.Fatalf("not ready %v", s.NotReady)
	}
	if len(s.Events) != 2 || s.Events[0].Reason != "Failed" || s.Events[0].Message != "secret-token" {
		t.Fatalf("events %+v", s.Events)
	}
}

func TestFromDoesNotInventUsage(t *testing.T) {
	s := From(map[string]*corev1.Node{
		"a": node("a", true, false, "2000m", "1Gi", "10"),
	}, nil, nil, cluster.Usage{}, nil, time.Now())
	if s.MetricsOK || s.CPU != "metrics unavailable" || s.Memory != "metrics unavailable" {
		t.Fatalf("%+v", s)
	}
	shown := Present(s, Worker, CPU, "15m", nil, time.Now())
	if shown.CPU.UsageOK || shown.Memory.UsageOK {
		t.Fatal("missing metrics still produced a usage")
	}
	if shown.CPU.Requests != 0 || shown.CPU.Allocatable != 2000 {
		t.Fatalf("spec totals changed %+v", shown.CPU)
	}
}

func TestPresentSplitsMasterAndDoesNotFillGaps(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	nodes := map[string]*corev1.Node{
		"cp": node("cp", true, true, "1000m", "1Gi", "4"),
		"wk": node("wk", true, false, "4000m", "4Gi", "20"),
	}
	pods := map[string]*corev1.Pod{
		"api": pod("api", "wk", corev1.PodRunning, "250m", "500m", "128Mi", "256Mi"),
		"sys": pod("sys", "cp", corev1.PodRunning, "100m", "100m", "64Mi", "64Mi"),
	}
	s := From(nodes, pods, nil, cluster.Usage{Available: true, CPUMilli: 300}, nil, now)
	samples := []Sample{{
		At: now.Add(-2 * time.Minute),
		Nodes: []cluster.NodeUsage{
			{Name: "wk", CPUMilli: 400, MemoryBytes: 512 * 1024 * 1024},
			{Name: "cp", CPUMilli: 80, MemoryBytes: 64 * 1024 * 1024},
		},
	}}
	worker := Present(s, Worker, CPU, "15m", samples, now)
	if !worker.HasMaster || !worker.HasWorker {
		t.Fatal("roles not detected")
	}
	if worker.CPU.Allocatable != 4000 || worker.CPU.Requests != 250 || worker.CPU.Limits != 500 {
		t.Fatalf("worker cpu %+v", worker.CPU)
	}
	if !worker.CPU.UsageOK || worker.CPU.Usage != 400 {
		t.Fatalf("worker usage %+v", worker.CPU)
	}
	if worker.Running != 1 {
		t.Fatalf("running %d", worker.Running)
	}
	master := Present(s, Master, CPU, "15m", samples, now)
	if master.CPU.Usage != 80 || master.CPU.Requests != 100 || master.CPU.Allocatable != 1000 {
		t.Fatalf("master leaked worker totals %+v", master.CPU)
	}
	old := Present(s, Worker, CPU, "1m", samples, now)
	if old.CPU.UsageOK || anyOK(old.History) {
		t.Fatal("a sample outside the window became a bar")
	}
	for _, p := range worker.History {
		if !p.OK {
			if !math.IsNaN(p.Value) {
				t.Fatal("empty bucket stored as zero")
			}
			continue
		}
		if p.Value == 0 {
			t.Fatal("filled a gap with zero")
		}
	}
}

func anyOK(points []Point) bool {
	for _, p := range points {
		if p.OK {
			return true
		}
	}
	return false
}

func node(name string, ready, master bool, cpu, mem, pods string) *corev1.Node {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	labels := map[string]string{}
	if master {
		labels["node-role.kubernetes.io/control-plane"] = ""
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(mem),
				corev1.ResourcePods:   resource.MustParse(pods),
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(mem),
				corev1.ResourcePods:   resource.MustParse(pods),
			},
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}},
		},
	}
}

func pod(name, nodeName string, phase corev1.PodPhase, reqCPU, limCPU, reqMem, limMem string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name: "app",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(reqCPU),
						corev1.ResourceMemory: resource.MustParse(reqMem),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(limCPU),
						corev1.ResourceMemory: resource.MustParse(limMem),
					},
				},
			}},
		},
		Status: corev1.PodStatus{Phase: phase},
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
