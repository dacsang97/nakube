package desktop

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Kind is one generic sidebar destination. Overview, workloads, Helm, Argo,
// Istio, and port-forwards are not in this map.
type Kind struct {
	ID  string
	GVR schema.GroupVersionResource
}

// GenericKinds is the locked long tail. A missing id fails the sidebar test.
func GenericKinds() []Kind {
	return []Kind{
		{ID: "nodes", GVR: schema.GroupVersionResource{Version: "v1", Resource: "nodes"}},
		{ID: "events", GVR: schema.GroupVersionResource{Version: "v1", Resource: "events"}},
		{ID: "namespaces", GVR: schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}},
		{ID: "leases", GVR: schema.GroupVersionResource{Group: "coordination.k8s.io", Version: "v1", Resource: "leases"}},
		{ID: "services", GVR: schema.GroupVersionResource{Version: "v1", Resource: "services"}},
		{ID: "ingresses", GVR: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}},
		{ID: "endpointslices", GVR: schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}},
		{ID: "networkpolicies", GVR: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}},
		{ID: "ingressclasses", GVR: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"}},
		{ID: "configmaps", GVR: schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}},
		{ID: "secrets", GVR: schema.GroupVersionResource{Version: "v1", Resource: "secrets"}},
		{ID: "horizontalpodautoscalers", GVR: schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}},
		{ID: "limitranges", GVR: schema.GroupVersionResource{Version: "v1", Resource: "limitranges"}},
		{ID: "resourcequotas", GVR: schema.GroupVersionResource{Version: "v1", Resource: "resourcequotas"}},
		{ID: "poddisruptionbudgets", GVR: schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}},
		{ID: "persistentvolumes", GVR: schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumes"}},
		{ID: "persistentvolumeclaims", GVR: schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}},
		{ID: "volumeattachments", GVR: schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "volumeattachments"}},
		{ID: "storageclasses", GVR: schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"}},
		{ID: "csidrivers", GVR: schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "csidrivers"}},
		{ID: "csinodes", GVR: schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "csinodes"}},
		{ID: "serviceaccounts", GVR: schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"}},
		{ID: "roles", GVR: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}},
		{ID: "rolebindings", GVR: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}},
		{ID: "clusterroles", GVR: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}},
		{ID: "clusterrolebindings", GVR: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"}},
		{ID: "crds", GVR: schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}},
		{ID: "priorityclasses", GVR: schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"}},
		{ID: "runtimeclasses", GVR: schema.GroupVersionResource{Group: "node.k8s.io", Version: "v1", Resource: "runtimeclasses"}},
		{ID: "mutatingwebhookconfigurations", GVR: schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingwebhookconfigurations"}},
		{ID: "validatingwebhookconfigurations", GVR: schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingwebhookconfigurations"}},
		{ID: "validatingadmissionpolicies", GVR: schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicies"}},
		{ID: "validatingadmissionpolicybindings", GVR: schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicybindings"}},
	}
}

// LookupKind returns the GVR for a generic sidebar id.
func LookupKind(id string) (Kind, bool) {
	for _, kind := range GenericKinds() {
		if kind.ID == id {
			return kind, true
		}
	}
	return Kind{}, false
}

// Row is the generic table row. Secret data is not a field.
type Row struct {
	Name      string
	Namespace string
	Age       time.Duration
}

// RowFromObject builds a table row. It reads only name, namespace, and age.
func RowFromObject(obj *unstructured.Unstructured, now time.Time) Row {
	if obj == nil {
		return Row{}
	}
	created := obj.GetCreationTimestamp().Time
	age := time.Duration(0)
	if !created.IsZero() {
		age = now.Sub(created)
	}
	return Row{Name: obj.GetName(), Namespace: obj.GetNamespace(), Age: age}
}
