package cluster

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// GenericRow is a list DTO. It has name, namespace, and age. It never carries
// Secret data or stringData.
type GenericRow struct {
	Name      string
	Namespace string
	Created   time.Time
}

// Age is the age of the row at now. A zero Created returns 0.
func (r GenericRow) Age(now time.Time) time.Duration {
	if r.Created.IsZero() {
		return 0
	}
	return now.Sub(r.Created)
}

// ListGeneric lists a GVR. Secret values are not copied into the row.
func (s *Session) ListGeneric(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]GenericRow, error) {
	dyn, err := s.dynamicClient()
	if err != nil {
		return nil, err
	}
	nri := dyn.Resource(gvr)
	var ri dynamic.ResourceInterface = nri
	if namespace != "" {
		ri = nri.Namespace(namespace)
	}
	list, err := ri.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return rowsFromList(list), nil
}

func rowsFromList(list *unstructured.UnstructuredList) []GenericRow {
	if list == nil {
		return nil
	}
	rows := make([]GenericRow, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		rows = append(rows, GenericRow{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Created:   item.GetCreationTimestamp().Time,
		})
	}
	return rows
}
