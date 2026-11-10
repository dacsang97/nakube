package pods

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	kit "github.com/dacsang97/nakube/internal/ui"
)

var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func pod(name string, mods ...func(*corev1.Pod)) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: name, CreationTimestamp: metav1.NewTime(t0)},
		Spec:       corev1.PodSpec{NodeName: "node-a", Containers: []corev1.Container{{Name: "app"}}},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBurstable,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
	for _, m := range mods {
		m(p)
	}
	return p
}

func waiting(reason string) func(*corev1.Pod) {
	return func(p *corev1.Pod) {
		p.Status.ContainerStatuses[0].Ready = false
		p.Status.ContainerStatuses[0].State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}}
	}
}

func exited(code int32, reason string) func(*corev1.Pod) {
	return func(p *corev1.Pod) {
		p.Status.Phase = corev1.PodSucceeded
		p.Status.ContainerStatuses[0].Ready = false
		p.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: code, Reason: reason}}
	}
}

func TestStatus(t *testing.T) {
	now := metav1.NewTime(t0)
	yes := true
	always := corev1.ContainerRestartPolicyAlways
	cases := []struct {
		name string
		pod  *corev1.Pod
		want string
		tone kit.Tone
	}{
		{"running", pod("a"), "Running", kit.ToneOK},
		{"crash", pod("a", waiting("CrashLoopBackOff")), "CrashLoopBackOff", kit.ToneBad},
		{"creating", pod("a", waiting("ContainerCreating")), "ContainerCreating", kit.ToneWarn},
		{"completed", pod("a", exited(0, "Completed")), "Completed", kit.ToneMuted},
		{"oom", pod("a", exited(137, "OOMKilled")), "OOMKilled", kit.ToneBad},
		{"exit code", pod("a", exited(2, "")), "ExitCode:2", kit.ToneBad},
		{"terminating", pod("a", func(p *corev1.Pod) { p.DeletionTimestamp = &now }), "Terminating", kit.ToneWarn},
		{"evicted", pod("a", func(p *corev1.Pod) {
			p.Status.Phase, p.Status.Reason = corev1.PodFailed, "Evicted"
			p.Status.ContainerStatuses = nil
		}), "Evicted", kit.ToneBad},
		{"init step", pod("a", func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodPending
			p.Spec.InitContainers = []corev1.Container{{Name: "migrate"}, {Name: "seed"}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{
				{Name: "migrate", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}},
				{Name: "seed", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			}
		}), "Init:1/2", kit.ToneWarn},
		{"init crash", pod("a", func(p *corev1.Pod) {
			p.Spec.InitContainers = []corev1.Container{{Name: "migrate"}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{
				{Name: "migrate", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}},
			}
		}), "Init:CrashLoopBackOff", kit.ToneBad},
		{"init running", pod("a", func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodPending
			p.Spec.InitContainers = []corev1.Container{{Name: "migrate"}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{
				{Name: "migrate", Started: &yes, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			}
			p.Status.ContainerStatuses[0].Ready = false
			p.Status.ContainerStatuses[0].State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}
		}), "Init:0/1", kit.ToneWarn},
		{"sidecar running", pod("a", func(p *corev1.Pod) {
			p.Spec.InitContainers = []corev1.Container{{Name: "proxy", RestartPolicy: &always}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{
				{Name: "proxy", Started: &yes, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
			}
		}), "Running", kit.ToneOK},
	}
	for _, c := range cases {
		got, tone := Status(c.pod)
		if got != c.want || tone != c.tone {
			t.Errorf("%s: got %q/%v, want %q/%v", c.name, got, tone, c.want, c.tone)
		}
	}
}

func TestRowFields(t *testing.T) {
	yes := true
	p := pod("web", func(p *corev1.Pod) {
		p.OwnerReferences = []metav1.OwnerReference{
			{APIVersion: "v1", Kind: "ConfigMap", Name: "x"},
			{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web-7d9", Controller: &yes},
		}
		p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: "proxy"})
		p.Status.ContainerStatuses[0].RestartCount = 2
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{
			Name: "proxy", RestartCount: 1, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		})
	})
	page := Build([]*corev1.Pod{p}, map[string]cluster.PodUsage{"shop/web": {CPUMilli: 302, MemoryBytes: 85 * 1024 * 1024}}, "", "", false)
	row := page.Rows[0]
	if row.Key != "shop/web" || row.Restarts != 3 || row.QoS != "Burstable" || row.Node != "node-a" {
		t.Fatalf("row %+v", row)
	}
	if row.Owner != (Owner{Kind: "ReplicaSet", Name: "web-7d9", Screen: "workloads", Key: "replicasets/shop/web-7d9"}) {
		t.Fatalf("owner %+v", row.Owner)
	}
	if len(row.Dots) != 2 || row.Dots[0].State != DotReady || row.Dots[1].State != DotFailed {
		t.Fatalf("dots %+v", row.Dots)
	}
	if !row.Known || CPU(row.CPU) != "0.302" || Memory(row.Mem) != "85MiB" {
		t.Fatalf("usage %v %v", CPU(row.CPU), Memory(row.Mem))
	}
}

func TestRestartsSkipInitSteps(t *testing.T) {
	always := corev1.ContainerRestartPolicyAlways
	p := pod("a", func(p *corev1.Pod) {
		p.Spec.InitContainers = []corev1.Container{{Name: "migrate"}, {Name: "proxy", RestartPolicy: &always}}
		p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "migrate", RestartCount: 4}, {Name: "proxy", RestartCount: 1}}
		p.Status.ContainerStatuses[0].RestartCount = 2
	})
	if got := restarts(p); got != 3 {
		t.Fatalf("restarts = %d, want app 2 + sidecar 1", got)
	}
}

func TestOwnerLinks(t *testing.T) {
	cases := []struct {
		ref    metav1.OwnerReference
		screen string
		key    string
	}{
		{metav1.OwnerReference{APIVersion: "batch/v1", Kind: "Job", Name: "j"}, "workloads", "jobs/shop/j"},
		{metav1.OwnerReference{APIVersion: "v1", Kind: "Node", Name: "node-a"}, "nodes", "node-a"},
		{metav1.OwnerReference{APIVersion: "argoproj.io/v1alpha1", Kind: "Rollout", Name: "r"}, "", ""},
		{metav1.OwnerReference{APIVersion: "example.com/v1", Kind: "ReplicaSet", Name: "r"}, "", ""},
	}
	for _, c := range cases {
		o := owner(pod("a", func(p *corev1.Pod) { p.OwnerReferences = []metav1.OwnerReference{c.ref} }))
		if o.Screen != c.screen || o.Key != c.key || o.Kind != c.ref.Kind {
			t.Errorf("%s: got %+v", c.ref.Kind, o)
		}
	}
	if o := owner(pod("bare")); o != (Owner{}) {
		t.Errorf("bare pod has an owner %+v", o)
	}
}

func TestDotsWithoutStatus(t *testing.T) {
	p := pod("new", func(p *corev1.Pod) {
		p.Spec.InitContainers = []corev1.Container{{Name: "migrate"}}
		p.Status.ContainerStatuses = nil
	})
	got := dots(p)
	if len(got) != 2 || !got[0].Init || got[0].State != DotWaiting || got[1].State != DotWaiting {
		t.Fatalf("dots %+v", got)
	}
	if got[0].Text != "migrate (init): waiting" {
		t.Fatalf("text %q", got[0].Text)
	}
}

func TestBuildSearchAndSort(t *testing.T) {
	older := pod("b", func(p *corev1.Pod) { p.CreationTimestamp = metav1.NewTime(t0.Add(-time.Hour)) })
	items := []*corev1.Pod{pod("a"), older, pod("c", func(p *corev1.Pod) { p.Spec.NodeName = "node-b" })}
	usage := map[string]cluster.PodUsage{"shop/a": {CPUMilli: 10}, "shop/c": {CPUMilli: 900}}

	if page := Build(items, usage, "node-b", "", false); len(page.Rows) != 1 || page.Total != 3 || page.Rows[0].Name != "c" {
		t.Fatalf("search by node %+v", page)
	}
	for _, desc := range []bool{false, true} {
		page := Build(items, usage, "", ColCPU, desc)
		if page.Rows[2].Name != "b" {
			t.Errorf("unknown CPU not last (desc=%v): %s", desc, page.Rows[2].Name)
		}
	}
	if page := Build(items, usage, "", ColCPU, true); page.Rows[0].Name != "c" {
		t.Errorf("highest CPU not first: %s", page.Rows[0].Name)
	}
	if page := Build(items, usage, "", ColAge, true); page.Rows[0].Name != "b" {
		t.Errorf("oldest not first by age desc: %s", page.Rows[0].Name)
	}
	if page := Build(items, nil, "", ColName, true); page.Rows[0].Name != "c" {
		t.Errorf("name desc: %s", page.Rows[0].Name)
	}
}

func TestFormats(t *testing.T) {
	if CPU(0) != "0.000" || CPU(1.5) != "1.500" {
		t.Fatal("cpu")
	}
	if Memory(1.5*1024*1024*1024) != "1.5GiB" || Memory(512) != "512B" || Memory(81.1*1024*1024) != "81.1MiB" {
		t.Fatalf("memory %s %s %s", Memory(1.5*1024*1024*1024), Memory(512), Memory(81.1*1024*1024))
	}
}
