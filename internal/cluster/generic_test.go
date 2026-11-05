package cluster

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestGenericRowHasNoSecretFields(t *testing.T) {
	typ := reflect.TypeOf(GenericRow{})
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		lower := strings.ToLower(name)
		if lower == "data" || lower == "stringdata" || strings.Contains(lower, "secret") {
			t.Fatalf("list DTO field %s must not exist", name)
		}
	}
}

func TestRowsFromListOmitSecretData(t *testing.T) {
	created := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	secret := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":              "db",
			"namespace":         "ns",
			"creationTimestamp": created.Format(time.RFC3339),
		},
		"data":       map[string]any{"password": "c2VjcmV0"},
		"stringData": map[string]any{"token": "super-secret"},
	}}
	rows := rowsFromList(&unstructured.UnstructuredList{Items: []unstructured.Unstructured{secret}})
	if len(rows) != 1 || rows[0].Name != "db" || rows[0].Namespace != "ns" {
		t.Fatalf("%+v", rows)
	}
	if rows[0].Age(created.Add(time.Hour)) < 59*time.Minute {
		t.Fatalf("age = %s", rows[0].Age(created.Add(time.Hour)))
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "c2VjcmV0") || strings.Contains(string(encoded), "super-secret") {
		t.Fatalf("row leaked secret values: %s", encoded)
	}
}

func TestListGenericOmitsSecretData(t *testing.T) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      "db",
			"namespace": "ns",
		},
		"data": map[string]any{"password": "c2VjcmV0"},
	}}
	dyn := fake.NewSimpleDynamicClient(runtime.NewScheme())
	dyn.PrependReactor("list", "secrets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, &unstructured.UnstructuredList{
			Object: map[string]any{"apiVersion": "v1", "kind": "SecretList"},
			Items:  []unstructured.Unstructured{*secret},
		}, nil
	})
	s := &Session{dyn: dyn}
	rows, err := s.ListGeneric(context.Background(), gvr, "ns")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(rows)
	if len(rows) != 1 || rows[0].Name != "db" || strings.Contains(string(encoded), "c2VjcmV0") {
		t.Fatalf("rows=%+v encoded=%s", rows, encoded)
	}
}
