package cluster

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func TestWatchedKinds(t *testing.T) {
	want := map[string]bool{
		"Pod": true, "Node": true, "Event": true,
		"Deployment": true, "ReplicaSet": true, "DaemonSet": true,
		"StatefulSet": true, "Job": true, "CronJob": true,
	}
	if len(WatchedKinds) != len(want) {
		t.Fatalf("kinds = %v", WatchedKinds)
	}
	for _, kind := range WatchedKinds {
		if !want[kind] {
			t.Fatalf("unexpected kind %s", kind)
		}
	}
}

func TestEmitCopiesBeforeCallback(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:   "p",
		Labels: map[string]string{"a": "1"},
	}}
	var got runtime.Object
	s := NewSession()
	s.watchGen = 1
	s.emit(1, func(ev WatchEvent) { got = ev.Object }, "add", "Pod", pod)
	pod.Labels["a"] = "changed"
	copied, ok := got.(*corev1.Pod)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if copied.Labels["a"] != "1" {
		t.Fatal("callback received the informer object")
	}
	copied.Labels["a"] = "from-callback"
	if pod.Labels["a"] != "changed" {
		t.Fatal("callback mutated the informer object")
	}
}

func TestEmitUnwrapsDeletedFinalState(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "gone"}}
	var got runtime.Object
	s := NewSession()
	s.watchGen = 3
	s.emit(3, func(ev WatchEvent) {
		if ev.Type != "delete" {
			t.Fatalf("type %s", ev.Type)
		}
		got = ev.Object
	}, "delete", "Pod", cache.DeletedFinalStateUnknown{Obj: pod})
	if got == nil || got.(*corev1.Pod).Name != "gone" {
		t.Fatalf("got %#v", got)
	}
}

func TestEmitDropsStaleGeneration(t *testing.T) {
	s := NewSession()
	s.watchGen = 2
	called := false
	s.emit(1, func(WatchEvent) { called = true }, "add", "Pod", &corev1.Pod{})
	if called {
		t.Fatal("stale watch delivered an event")
	}
}
