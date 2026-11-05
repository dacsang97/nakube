package desktop

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGenericMapCoversSidebar(t *testing.T) {
	ids := map[string]bool{}
	for _, id := range SidebarIDs() {
		ids[id] = true
	}
	for _, forbidden := range []string{"resource-diagram", "flux", "overview", "workloads", "helm", "port-forwards"} {
		if _, ok := LookupKind(forbidden); ok {
			t.Fatalf("%s is in the GVR map", forbidden)
		}
	}
	for _, kind := range GenericKinds() {
		if !ids[kind.ID] {
			t.Fatalf("sidebar missing %s", kind.ID)
		}
		if kind.GVR.Resource == "" {
			t.Fatalf("%s has no resource", kind.ID)
		}
	}
	if _, ok := LookupKind("leases"); !ok {
		t.Fatal("leases missing from the GVR map")
	}
}

func TestLeaseRowDoesNotPanic(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "coordination.k8s.io/v1",
		"kind":       "Lease",
		"metadata": map[string]any{
			"name":              "kube-controller-manager",
			"namespace":         "kube-system",
			"creationTimestamp": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
		},
		"spec": map[string]any{"holderIdentity": "node-a"},
	}}
	row := RowFromObject(obj, time.Now())
	if row.Name != "kube-controller-manager" || row.Namespace != "kube-system" || row.Age <= 0 {
		t.Fatalf("%+v", row)
	}
}
