package desktop

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/resource"
)

func TestActionsFollowKind(t *testing.T) {
	pods := resource.Actions("Pod")
	if !hasAction(pods, "logs") || !hasAction(pods, "exec") || hasAction(pods, "sync") {
		t.Fatalf("pod actions = %v", pods)
	}
	deps := resource.Actions("Deployment")
	if !hasAction(deps, "scale") || !hasAction(deps, "restart") || hasAction(deps, "logs") {
		t.Fatalf("deployment actions = %v", deps)
	}
	if resource.Actions("") != nil {
		t.Fatal("summary rows should have no actions")
	}
}

func TestPodTableUsesTheWatch(t *testing.T) {
	a := New(nil)
	a.pods["ns/web"] = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	_, rows := a.liveRowsLocked("pods")
	if len(rows) != 1 || rows[0].Name != "web" || rows[0].Identity.Kind != "Pod" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Cells[2] != "Running" {
		t.Fatalf("phase = %q", rows[0].Cells[2])
	}
}

func TestAskCapturesReplicas(t *testing.T) {
	a := New(nil)
	a.scaleTo = "3"
	a.ask(listRow{Namespace: "ns", Name: "web", Identity: cluster.Identity{Kind: "Deployment"}}, "scale")
	got, ok := a.ConfirmIdentity()
	if !ok || got.Replicas != 3 || got.Name != "web" {
		t.Fatalf("confirm = %+v ok=%v", got, ok)
	}
	a.scaleTo = "nope"
	a.ask(listRow{Name: "web", Identity: cluster.Identity{Kind: "Deployment"}}, "scale")
	if a.errText == "" {
		t.Fatal("bad replica count was accepted")
	}
}

func hasAction(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}
