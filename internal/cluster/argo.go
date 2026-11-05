package cluster

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const argoFinalizer = "resources-finalizer.argocd.argoproj.io"

// ArgoSyncOptions is a confirmed sync. Empty revision is HEAD. Empty strategy is hook.
type ArgoSyncOptions struct {
	Revision string
	Strategy string
	DryRun   bool
	Prune    bool
}

// ArgoApplication is a phone-safe read. It has no operation field.
type ArgoApplication struct {
	Name      string
	Namespace string
	Sync      string
	Health    string
}

func argoGVR(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: resource}
}

// BuildArgoSyncPatch returns the Application sync patch. dryRun is always present.
func BuildArgoSyncPatch(opts ArgoSyncOptions) ([]byte, error) {
	revision := opts.Revision
	if revision == "" {
		revision = "HEAD"
	}
	strategy := opts.Strategy
	if strategy == "" || strategy != "apply" {
		strategy = "hook"
	}
	strategyBody := map[string]any{}
	sync := map[string]any{
		"dryRun":   opts.DryRun,
		"prune":    opts.Prune,
		"revision": revision,
		"syncStrategy": map[string]any{
			strategy: strategyBody,
		},
	}
	body := map[string]any{
		"operation": map[string]any{
			"initiatedBy": map[string]any{"automated": false, "username": "nakube"},
			"sync":        sync,
		},
	}
	return json.Marshal(body)
}

// BuildArgoRollbackPatch returns the rollback operation patch.
func BuildArgoRollbackPatch(id int64, prune bool) ([]byte, error) {
	body := map[string]any{
		"operation": map[string]any{
			"initiatedBy": map[string]any{"automated": false, "username": "nakube"},
			"rollback":    map[string]any{"id": id, "prune": prune},
		},
	}
	return json.Marshal(body)
}

// SyncArgo patches an Application. It is not a phone method.
func (s *Session) SyncArgo(ctx context.Context, namespace, name string, opts ArgoSyncOptions) error {
	data, err := BuildArgoSyncPatch(opts)
	if err != nil {
		return err
	}
	return s.patchArgo(ctx, "applications", namespace, name, data)
}

// RollbackArgo patches an Application back to a history id.
func (s *Session) RollbackArgo(ctx context.Context, namespace, name string, id int64, prune bool) error {
	data, err := BuildArgoRollbackPatch(id, prune)
	if err != nil {
		return err
	}
	return s.patchArgo(ctx, "applications", namespace, name, data)
}

func (s *Session) patchArgo(ctx context.Context, resource, namespace, name string, data []byte) error {
	dyn, err := s.dynamicClient()
	if err != nil {
		return err
	}
	_, err = resourceFor(dyn, argoGVR(resource), namespace).Patch(ctx, name, types.MergePatchType, data, metav1.PatchOptions{FieldManager: fieldManager})
	if err != nil {
		return fmt.Errorf("argo %s %s/%s: %w", resource, namespace, name, err)
	}
	return nil
}

// DeleteArgo deletes an Application. cascade is foreground, background, or non-cascading.
func (s *Session) DeleteArgo(ctx context.Context, namespace, name, cascade string) error {
	policy, keepFinalizer, err := argoCascade(cascade)
	if err != nil {
		return err
	}
	dyn, err := s.dynamicClient()
	if err != nil {
		return err
	}
	ri := resourceFor(dyn, argoGVR("applications"), namespace)
	obj, err := ri.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if err := setArgoFinalizer(obj, keepFinalizer); err != nil {
		return err
	}
	if _, err := ri.Update(ctx, obj, metav1.UpdateOptions{FieldManager: fieldManager}); err != nil && !apierrors.IsConflict(err) {
		return fmt.Errorf("set finalizer on application %s: %w", name, err)
	}
	return ri.Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy})
}

func argoCascade(cascade string) (metav1.DeletionPropagation, bool, error) {
	switch cascade {
	case "", "foreground":
		return metav1.DeletePropagationForeground, true, nil
	case "background":
		return metav1.DeletePropagationBackground, true, nil
	case "non-cascading":
		return metav1.DeletePropagationOrphan, false, nil
	default:
		return "", false, fmt.Errorf("invalid cascade mode: %q (want foreground|background|non-cascading)", cascade)
	}
}

func setArgoFinalizer(obj *unstructured.Unstructured, keep bool) error {
	finalizers := obj.GetFinalizers()
	has := false
	out := make([]string, 0, len(finalizers)+1)
	for _, f := range finalizers {
		if f == argoFinalizer {
			has = true
			if keep {
				out = append(out, f)
			}
			continue
		}
		out = append(out, f)
	}
	if keep && !has {
		out = append(out, argoFinalizer)
	}
	obj.SetFinalizers(out)
	return nil
}

// ListArgoApplications reads Application status for the phone.
// A missing CRD is a NotFound error. The row has no operation field.
func (s *Session) ListArgoApplications(ctx context.Context) ([]ArgoApplication, error) {
	apps, err := s.ListArgoApps(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ArgoApplication, len(apps))
	for i, app := range apps {
		out[i] = ArgoApplication{Name: app.Name, Namespace: app.Namespace, Sync: app.Sync, Health: app.Health}
	}
	return out, nil
}
