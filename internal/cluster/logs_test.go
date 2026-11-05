package cluster

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTagLinesCopiesBatch(t *testing.T) {
	batch := []string{"hello", "world"}
	got := tagLines("web-0", "app", batch)
	batch[0] = "mutated"
	if len(got) != 2 || got[0].Pod != "web-0" || got[0].Container != "app" || got[0].Text != "hello" {
		t.Fatalf("%+v", got)
	}
	if got[1].Text != "world" {
		t.Fatalf("%+v", got[1])
	}
}

func TestDeliverLinesOwnsBatch(t *testing.T) {
	src := []string{"a", "b"}
	var got []string
	deliverLines(src, func(lines []string) { got = lines })
	src[0] = "mutated"
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("callback saw reused batch: %v", got)
	}
}

func TestFanoutTargetsTagsEachContainer(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web-0"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: "proxy"}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web-1"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
		},
	}
	targets := fanoutTargets(pods)
	if len(targets) != 3 {
		t.Fatalf("targets = %+v", targets)
	}
	batch := []string{"ready"}
	line := tagLines(targets[0].pod, targets[0].container, batch)[0]
	batch[0] = "changed"
	if line.Pod != "web-0" || line.Container != "app" || line.Text != "ready" {
		t.Fatalf("%+v", line)
	}
}
