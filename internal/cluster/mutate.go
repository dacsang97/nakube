package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

const fieldManager = "nakube"

// Identity is the object a confirm dialog captured when it opened.
// Later watch updates must not retarget the call.
type Identity struct {
	GVR       schema.GroupVersionResource
	Kind      string
	Namespace string
	Name      string
}

// ApplyPreview is the server-side dry-run result. Live is empty on create.
type ApplyPreview struct {
	Identity  Identity
	Action    string
	Live      string
	Predicted string
}

// DryRunApply runs a server-side apply with DryRun=All. It does not write.
func (s *Session) DryRunApply(ctx context.Context, yamlBody string) (*ApplyPreview, error) {
	obj, gvk, data, err := parseApplyBody(yamlBody)
	if err != nil {
		return nil, err
	}
	gvr, err := s.resolveGVK(gvk)
	if err != nil {
		return nil, err
	}
	dyn, err := s.dynamicClient()
	if err != nil {
		return nil, err
	}
	ri := resourceFor(dyn, gvr, obj.GetNamespace())

	live := ""
	action := "create"
	cur, err := ri.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err == nil {
		action = "update"
		live, err = sanitizedYAML(cur)
		if err != nil {
			return nil, err
		}
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	force := true
	predicted, err := ri.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: fieldManager,
		Force:        &force,
		DryRun:       []string{metav1.DryRunAll},
	})
	if err != nil {
		return nil, err
	}
	after, err := sanitizedYAML(predicted)
	if err != nil {
		return nil, err
	}
	return &ApplyPreview{
		Identity: Identity{
			GVR:       gvr,
			Kind:      gvk.Kind,
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
		Action:    action,
		Live:      live,
		Predicted: after,
	}, nil
}

// Apply writes yamlBody. id is the identity captured when the dialog opened.
// A mismatch is refused so a later selection cannot retarget the write.
func (s *Session) Apply(ctx context.Context, id Identity, yamlBody string) error {
	obj, gvk, data, err := parseApplyBody(yamlBody)
	if err != nil {
		return err
	}
	if err := identityMatches(id, obj, gvk.Kind); err != nil {
		return err
	}
	gvr, err := s.resolveGVK(gvk)
	if err != nil {
		return err
	}
	dyn, err := s.dynamicClient()
	if err != nil {
		return err
	}
	force := true
	_, err = resourceFor(dyn, gvr, obj.GetNamespace()).Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: fieldManager,
		Force:        &force,
	})
	return err
}

// Delete removes the captured object. It does not look at the current selection.
func (s *Session) Delete(ctx context.Context, id Identity) error {
	if id.Name == "" {
		return errors.New("delete requires a name")
	}
	gvr, err := s.gvrForIdentity(id)
	if err != nil {
		return err
	}
	dyn, err := s.dynamicClient()
	if err != nil {
		return err
	}
	return resourceFor(dyn, gvr, id.Namespace).Delete(ctx, id.Name, metav1.DeleteOptions{})
}

// Scale sets replicas on a Deployment, StatefulSet, or ReplicaSet.
func (s *Session) Scale(ctx context.Context, id Identity, replicas int32) error {
	if id.Name == "" {
		return errors.New("scale requires a name")
	}
	cs, err := s.clientset()
	if err != nil {
		return err
	}
	switch id.Kind {
	case "Deployment":
		scale, err := cs.AppsV1().Deployments(id.Namespace).GetScale(ctx, id.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		scale.Spec.Replicas = replicas
		_, err = cs.AppsV1().Deployments(id.Namespace).UpdateScale(ctx, id.Name, scale, metav1.UpdateOptions{})
		return err
	case "StatefulSet":
		scale, err := cs.AppsV1().StatefulSets(id.Namespace).GetScale(ctx, id.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		scale.Spec.Replicas = replicas
		_, err = cs.AppsV1().StatefulSets(id.Namespace).UpdateScale(ctx, id.Name, scale, metav1.UpdateOptions{})
		return err
	case "ReplicaSet":
		scale, err := cs.AppsV1().ReplicaSets(id.Namespace).GetScale(ctx, id.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		scale.Spec.Replicas = replicas
		_, err = cs.AppsV1().ReplicaSets(id.Namespace).UpdateScale(ctx, id.Name, scale, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("scale not supported for kind %q", id.Kind)
	}
}

// Restart patches the pod template of a Deployment, StatefulSet, or DaemonSet.
// Kind Pod is rejected: a phone restart deletes an owned pod instead.
func (s *Session) Restart(ctx context.Context, id Identity) error {
	switch id.Kind {
	case "Deployment", "StatefulSet", "DaemonSet":
	default:
		return fmt.Errorf("restart not supported for kind %q", id.Kind)
	}
	if id.Name == "" {
		return errors.New("restart requires a name")
	}
	gvr, err := s.gvrForIdentity(id)
	if err != nil {
		return err
	}
	dyn, err := s.dynamicClient()
	if err != nil {
		return err
	}
	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"kubectl.kubernetes.io/restartedAt": time.Now().UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = resourceFor(dyn, gvr, id.Namespace).Patch(ctx, id.Name, types.StrategicMergePatchType, data, metav1.PatchOptions{
		FieldManager: fieldManager,
	})
	return err
}

// GetYAML returns sanitized object YAML for the editor. Callers mask secrets.
func (s *Session) GetYAML(ctx context.Context, id Identity) (string, error) {
	gvr, err := s.gvrForIdentity(id)
	if err != nil {
		return "", err
	}
	dyn, err := s.dynamicClient()
	if err != nil {
		return "", err
	}
	obj, err := resourceFor(dyn, gvr, id.Namespace).Get(ctx, id.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return sanitizedYAML(obj)
}

func parseApplyBody(yamlBody string) (*unstructured.Unstructured, schema.GroupVersionKind, []byte, error) {
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(yamlBody), &obj.Object); err != nil {
		return nil, schema.GroupVersionKind{}, nil, errors.New("invalid YAML")
	}
	gvk := obj.GroupVersionKind()
	if gvk.Kind == "" {
		return nil, schema.GroupVersionKind{}, nil, errors.New("YAML missing kind")
	}
	data, err := yaml.YAMLToJSON([]byte(yamlBody))
	if err != nil {
		return nil, schema.GroupVersionKind{}, nil, errors.New("invalid YAML")
	}
	return obj, gvk, data, nil
}

func identityMatches(id Identity, obj *unstructured.Unstructured, kind string) error {
	if id.Name != "" && id.Name != obj.GetName() {
		return fmt.Errorf("apply target %s does not match YAML name %s", id.Name, obj.GetName())
	}
	if id.Namespace != "" && id.Namespace != obj.GetNamespace() {
		return fmt.Errorf("apply target namespace %s does not match YAML", id.Namespace)
	}
	if id.Kind != "" && id.Kind != kind {
		return fmt.Errorf("apply target kind %s does not match YAML kind %s", id.Kind, kind)
	}
	return nil
}

func resourceFor(d dynamic.Interface, gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		return d.Resource(gvr)
	}
	return d.Resource(gvr).Namespace(namespace)
}

func sanitizedYAML(obj *unstructured.Unstructured) (string, error) {
	clone := obj.DeepCopy()
	unstructured.RemoveNestedField(clone.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(clone.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(clone.Object, "metadata", "generation")
	unstructured.RemoveNestedField(clone.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(clone.Object, "metadata", "uid")
	unstructured.RemoveNestedField(clone.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(clone.Object, "metadata", "ownerReferences")
	unstructured.RemoveNestedField(clone.Object, "status")
	data, err := yaml.Marshal(clone.Object)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Session) resolveGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	s.mu.Lock()
	mapper := s.mapper
	s.mu.Unlock()
	if mapper != nil {
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err == nil {
			return mapping.Resource, nil
		}
	}
	if gvr, ok := builtinGVR(gvk); ok {
		return gvr, nil
	}
	return schema.GroupVersionResource{}, fmt.Errorf("unsupported kind: %q", gvk.Kind)
}

func (s *Session) gvrForIdentity(id Identity) (schema.GroupVersionResource, error) {
	if id.GVR.Resource != "" {
		return id.GVR, nil
	}
	if id.Kind == "" {
		return schema.GroupVersionResource{}, errors.New("resource kind is required")
	}
	return s.resolveGVK(schema.GroupVersionKind{Kind: id.Kind})
}

func builtinGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool) {
	gvr, ok := builtinKindGVR[gvk.Kind]
	if !ok {
		return schema.GroupVersionResource{}, false
	}
	if gvk.Group != "" && gvr.Group != "" && gvk.Group != gvr.Group {
		return schema.GroupVersionResource{}, false
	}
	if gvk.Version != "" {
		gvr.Version = gvk.Version
	}
	return gvr, true
}

var builtinKindGVR = map[string]schema.GroupVersionResource{
	"Pod":         {Version: "v1", Resource: "pods"},
	"Node":        {Version: "v1", Resource: "nodes"},
	"Event":       {Version: "v1", Resource: "events"},
	"ConfigMap":   {Version: "v1", Resource: "configmaps"},
	"Secret":      {Version: "v1", Resource: "secrets"},
	"Service":     {Version: "v1", Resource: "services"},
	"Namespace":   {Version: "v1", Resource: "namespaces"},
	"Deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
	"ReplicaSet":  {Group: "apps", Version: "v1", Resource: "replicasets"},
	"DaemonSet":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"StatefulSet": {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"Job":         {Group: "batch", Version: "v1", Resource: "jobs"},
	"CronJob":     {Group: "batch", Version: "v1", Resource: "cronjobs"},
}
