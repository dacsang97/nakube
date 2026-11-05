package cluster

import (
	"context"
	"strings"
	"testing"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

const configMapYAML = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
  namespace: ns
data:
  k: v
`

func TestDryRunApplyDoesNotMutate(t *testing.T) {
	live := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "ns",
		},
		"data": map[string]any{"k": "old"},
	}}
	predicted := live.DeepCopy()
	_ = unstructured.SetNestedField(predicted.Object, "v", "data", "k")

	mutated := false
	dyn := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dyn.PrependReactor("*", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		switch action.GetVerb() {
		case "get":
			return true, live.DeepCopy(), nil
		case "create", "update", "delete", "delete-collection":
			mutated = true
			return true, nil, errMutating(action.GetVerb())
		case "patch":
			opts := patchOptions(t, action)
			if len(opts.DryRun) == 0 {
				mutated = true
				return true, nil, errMutating("patch")
			}
			if len(opts.DryRun) != 1 || opts.DryRun[0] != metav1.DryRunAll {
				t.Errorf("DryRun = %#v", opts.DryRun)
			}
			if opts.FieldManager != fieldManager || opts.Force == nil || !*opts.Force {
				t.Errorf("patch options = %+v", opts)
			}
			pa := action.(clienttesting.PatchAction)
			if pa.GetPatchType() != types.ApplyPatchType {
				t.Errorf("patch type %s", pa.GetPatchType())
			}
			return true, predicted.DeepCopy(), nil
		default:
			return false, nil, nil
		}
	})

	s := &Session{dyn: dyn}
	preview, err := s.DryRunApply(context.Background(), configMapYAML)
	if err != nil {
		t.Fatal(err)
	}
	if mutated {
		t.Fatal("dry-run took the mutating branch")
	}
	if preview.Action != "update" {
		t.Fatalf("action %s", preview.Action)
	}
	if !strings.Contains(preview.Live, "old") || !strings.Contains(preview.Predicted, "v") {
		t.Fatalf("live=%q predicted=%q", preview.Live, preview.Predicted)
	}
	if preview.Identity.Name != "demo" || preview.Identity.Namespace != "ns" || preview.Identity.Kind != "ConfigMap" {
		t.Fatalf("identity %+v", preview.Identity)
	}
}

func TestApplyTakesMutatingBranch(t *testing.T) {
	applied := false
	dyn := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dyn.PrependReactor("patch", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		opts := patchOptions(t, action)
		if len(opts.DryRun) != 0 {
			t.Fatalf("apply set DryRun %#v", opts.DryRun)
		}
		applied = true
		return true, &unstructured.Unstructured{Object: map[string]any{"kind": "ConfigMap"}}, nil
	})
	s := &Session{dyn: dyn}
	err := s.Apply(context.Background(), Identity{Kind: "ConfigMap", Namespace: "ns", Name: "demo"}, configMapYAML)
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("apply did not patch")
	}
}

func TestApplyRefusesStaleIdentity(t *testing.T) {
	s := &Session{}
	err := s.Apply(context.Background(), Identity{Kind: "ConfigMap", Namespace: "ns", Name: "other"}, configMapYAML)
	if err == nil {
		t.Fatal("expected identity mismatch")
	}
	if strings.Contains(err.Error(), "k: v") {
		t.Fatalf("error includes YAML body: %v", err)
	}
}

func TestRestartRejectsPod(t *testing.T) {
	s := NewSession()
	err := s.Restart(context.Background(), Identity{Kind: "Pod", Namespace: "ns", Name: "p"})
	if err == nil || !strings.Contains(err.Error(), "Pod") {
		t.Fatal(err)
	}
}

func TestScaleDeployment(t *testing.T) {
	cs := k8sfake.NewClientset()
	var got int32
	cs.PrependReactor("get", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		return true, &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "ns"},
			Spec:       autoscalingv1.ScaleSpec{Replicas: 1},
		}, nil
	})
	cs.PrependReactor("update", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "scale" {
			return false, nil, nil
		}
		scale := action.(clienttesting.UpdateAction).GetObject().(*autoscalingv1.Scale)
		got = scale.Spec.Replicas
		return true, scale, nil
	})
	s := &Session{cs: cs}
	if err := s.Scale(context.Background(), Identity{Kind: "Deployment", Namespace: "ns", Name: "web"}, 3); err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("replicas = %d", got)
	}
}

func TestDeleteUsesCapturedIdentity(t *testing.T) {
	var gotName, gotNS string
	dyn := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dyn.PrependReactor("delete", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		da := action.(clienttesting.DeleteAction)
		gotName = da.GetName()
		gotNS = da.GetNamespace()
		return true, nil, nil
	})
	s := &Session{dyn: dyn}
	err := s.Delete(context.Background(), Identity{
		GVR:       schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
		Namespace: "ns",
		Name:      "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "demo" || gotNS != "ns" {
		t.Fatalf("deleted %s/%s", gotNS, gotName)
	}
}

func patchOptions(t *testing.T, action clienttesting.Action) metav1.PatchOptions {
	t.Helper()
	impl, ok := action.(clienttesting.PatchActionImpl)
	if !ok {
		t.Fatalf("patch action %T", action)
	}
	return impl.GetPatchOptions()
}

func errMutating(verb string) error {
	return &mutateError{verb: verb}
}

type mutateError struct{ verb string }

func (e *mutateError) Error() string { return "mutating " + e.verb }
