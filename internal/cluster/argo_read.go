package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const argoRefreshAnnotation = "argocd.argoproj.io/refresh"

// ArgoApp is a desktop read of one Application. It has no operation request
// the phone could post back. Phone rows stay on ArgoApplication.
type ArgoApp struct {
	Name             string
	Namespace        string
	Project          string
	Sync             string
	Health           string
	HealthMessage    string
	Revision         string
	TargetRevision   string
	Destination      string
	Policy           string
	Source           string
	Sources          []ArgoSource
	Resources        []ArgoResource
	History          []ArgoHistory
	Conditions       []string
	OperationPhase   string
	OperationMessage string
	OperationBy      string
	Images           []string
	SourceType       string
	Created          time.Time
}

// ArgoSource is one spec.source or spec.sources entry.
type ArgoSource struct {
	RepoURL        string
	Path           string
	Chart          string
	TargetRevision string
	Ref            string
}

// ArgoResource is one status.resources entry.
type ArgoResource struct {
	Kind      string
	Namespace string
	Name      string
	Sync      string
	Health    string
	Hook      bool
}

// ArgoHistory is one status.history entry.
type ArgoHistory struct {
	ID       int64
	Revision string
	By       string
}

// ArgoApplicationSet is a desktop read. Generators are type names only.
type ArgoApplicationSet struct {
	Name        string
	Namespace   string
	Generators  string
	Project     string
	Destination string
	Conditions  []string
	Created     time.Time
}

// ArgoProject is a desktop read of an AppProject.
type ArgoProject struct {
	Name         string
	Namespace    string
	Description  string
	SourceRepos  []string
	Destinations []string
	Roles        int
	Created      time.Time
}

// ArgoRepository is a label-selected Secret. Credential keys are not copied.
type ArgoRepository struct {
	Name      string
	Namespace string
	Type      string
	URL       string
	Project   string
}

// ArgoCluster is a label-selected cluster Secret. The in-cluster local
// cluster has no Secret and is absent. Credential keys are not copied.
type ArgoCluster struct {
	Name      string
	Namespace string
	Server    string
}

// ParseArgoApp reads Application fields. A nil object is empty, not an error.
func ParseArgoApp(obj *unstructured.Unstructured) ArgoApp {
	if obj == nil {
		return ArgoApp{}
	}
	sources := argoSources(obj.Object)
	target := ""
	if len(sources) > 0 {
		target = sources[0].TargetRevision
	}
	app := ArgoApp{
		Name:             obj.GetName(),
		Namespace:        obj.GetNamespace(),
		Project:          nestedString(obj.Object, "spec", "project"),
		Sync:             nestedString(obj.Object, "status", "sync", "status"),
		Health:           nestedString(obj.Object, "status", "health", "status"),
		HealthMessage:    nestedString(obj.Object, "status", "health", "message"),
		Revision:         nestedString(obj.Object, "status", "sync", "revision"),
		TargetRevision:   target,
		Destination:      argoDestination(obj.Object, "spec", "destination"),
		Policy:           argoPolicy(obj.Object),
		Sources:          sources,
		Source:           argoSourceLine(sources),
		Resources:        argoResources(obj.Object),
		History:          argoHistory(obj.Object),
		Conditions:       argoConditions(obj.Object, "status", "conditions"),
		OperationPhase:   nestedString(obj.Object, "status", "operationState", "phase"),
		OperationMessage: nestedString(obj.Object, "status", "operationState", "message"),
		OperationBy:      argoInitiatedBy(obj.Object, "status", "operationState", "operation", "initiatedBy"),
		Images:           nestedStrings(obj.Object, "status", "summary", "images"),
		SourceType:       nestedString(obj.Object, "status", "sourceType"),
		Created:          obj.GetCreationTimestamp().Time,
	}
	return app
}

// ParseArgoApplicationSet reads generator type names and the template destination.
func ParseArgoApplicationSet(obj *unstructured.Unstructured) ArgoApplicationSet {
	if obj == nil {
		return ArgoApplicationSet{}
	}
	return ArgoApplicationSet{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		Generators:  argoGeneratorTypes(obj.Object),
		Project:     nestedString(obj.Object, "spec", "template", "spec", "project"),
		Destination: argoDestination(obj.Object, "spec", "template", "spec", "destination"),
		Conditions:  argoConditions(obj.Object, "status", "conditions"),
		Created:     obj.GetCreationTimestamp().Time,
	}
}

// ParseArgoProject reads description, source repos, and destinations.
func ParseArgoProject(obj *unstructured.Unstructured) ArgoProject {
	if obj == nil {
		return ArgoProject{}
	}
	dests := argoProjectDestinations(obj.Object)
	roles, _, _ := unstructured.NestedSlice(obj.Object, "spec", "roles")
	return ArgoProject{
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		Description:  nestedString(obj.Object, "spec", "description"),
		SourceRepos:  nestedStrings(obj.Object, "spec", "sourceRepos"),
		Destinations: dests,
		Roles:        len(roles),
		Created:      obj.GetCreationTimestamp().Time,
	}
}

// ParseArgoRepository reads the non-secret repository fields.
func ParseArgoRepository(obj *unstructured.Unstructured) ArgoRepository {
	if obj == nil {
		return ArgoRepository{}
	}
	kind := decodeSecret(obj, "type")
	if kind == "" {
		kind = "git"
	}
	name := decodeSecret(obj, "name")
	if name == "" {
		name = obj.GetName()
	}
	return ArgoRepository{
		Name:      name,
		Namespace: obj.GetNamespace(),
		Type:      kind,
		URL:       decodeSecret(obj, "url"),
		Project:   decodeSecret(obj, "project"),
	}
}

// ParseArgoCluster reads the non-secret cluster fields.
func ParseArgoCluster(obj *unstructured.Unstructured) ArgoCluster {
	if obj == nil {
		return ArgoCluster{}
	}
	name := decodeSecret(obj, "name")
	if name == "" {
		name = obj.GetName()
	}
	return ArgoCluster{
		Name:      name,
		Namespace: obj.GetNamespace(),
		Server:    decodeSecret(obj, "server"),
	}
}

// BuildArgoRefreshPatch asks the controller to compare the Application again.
// kind is normal or hard. Hard also drops the manifest cache.
func BuildArgoRefreshPatch(kind string) ([]byte, error) {
	if kind != "normal" && kind != "hard" {
		return nil, fmt.Errorf("invalid refresh %q (want normal|hard)", kind)
	}
	body := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				argoRefreshAnnotation: kind,
			},
		},
	}
	return json.Marshal(body)
}

// ListArgoApps reads Applications. A missing CRD is a NotFound error.
func (s *Session) ListArgoApps(ctx context.Context) ([]ArgoApp, error) {
	items, err := s.listArgo(ctx, "applications", "")
	if err != nil {
		return nil, err
	}
	out := make([]ArgoApp, 0, len(items))
	for i := range items {
		out = append(out, ParseArgoApp(&items[i]))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out, nil
}

// ListArgoApplicationSets reads ApplicationSets. A missing CRD is a NotFound error.
func (s *Session) ListArgoApplicationSets(ctx context.Context) ([]ArgoApplicationSet, error) {
	items, err := s.listArgo(ctx, "applicationsets", "")
	if err != nil {
		return nil, err
	}
	out := make([]ArgoApplicationSet, 0, len(items))
	for i := range items {
		out = append(out, ParseArgoApplicationSet(&items[i]))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out, nil
}

// ListArgoProjects reads AppProjects. A missing CRD is a NotFound error.
func (s *Session) ListArgoProjects(ctx context.Context) ([]ArgoProject, error) {
	items, err := s.listArgo(ctx, "appprojects", "")
	if err != nil {
		return nil, err
	}
	out := make([]ArgoProject, 0, len(items))
	for i := range items {
		out = append(out, ParseArgoProject(&items[i]))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out, nil
}

// ListArgoRepositories reads repository Secrets. It does not copy credentials.
func (s *Session) ListArgoRepositories(ctx context.Context) ([]ArgoRepository, error) {
	items, err := s.listSecrets(ctx, "argocd.argoproj.io/secret-type=repository")
	if err != nil {
		return nil, err
	}
	out := make([]ArgoRepository, 0, len(items))
	for i := range items {
		out = append(out, ParseArgoRepository(&items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListArgoClusters reads cluster Secrets. The local cluster is not in this list.
func (s *Session) ListArgoClusters(ctx context.Context) ([]ArgoCluster, error) {
	items, err := s.listSecrets(ctx, "argocd.argoproj.io/secret-type=cluster")
	if err != nil {
		return nil, err
	}
	out := make([]ArgoCluster, 0, len(items))
	for i := range items {
		out = append(out, ParseArgoCluster(&items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// RefreshArgo sets the refresh annotation. It is not a phone method.
func (s *Session) RefreshArgo(ctx context.Context, namespace, name, kind string) error {
	data, err := BuildArgoRefreshPatch(kind)
	if err != nil {
		return err
	}
	return s.patchArgo(ctx, "applications", namespace, name, data)
}

func (s *Session) listArgo(ctx context.Context, resource, selector string) ([]unstructured.Unstructured, error) {
	dyn, err := s.dynamicClient()
	if err != nil {
		return nil, err
	}
	list, err := dyn.Resource(argoGVR(resource)).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *Session) listSecrets(ctx context.Context, selector string) ([]unstructured.Unstructured, error) {
	dyn, err := s.dynamicClient()
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	list, err := dyn.Resource(gvr).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func argoSources(obj map[string]any) []ArgoSource {
	items, _, _ := unstructured.NestedSlice(obj, "spec", "sources")
	if len(items) > 0 {
		out := make([]ArgoSource, 0, len(items))
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, argoSource(m))
		}
		return out
	}
	if src, ok, _ := unstructured.NestedMap(obj, "spec", "source"); ok {
		return []ArgoSource{argoSource(src)}
	}
	return nil
}

func argoSource(m map[string]any) ArgoSource {
	return ArgoSource{
		RepoURL:        nestedString(m, "repoURL"),
		Path:           nestedString(m, "path"),
		Chart:          nestedString(m, "chart"),
		TargetRevision: nestedString(m, "targetRevision"),
		Ref:            nestedString(m, "ref"),
	}
}

func argoSourceLine(sources []ArgoSource) string {
	parts := make([]string, 0, len(sources))
	for _, src := range sources {
		line := src.RepoURL
		switch {
		case src.Path != "":
			line = joinNonEmpty(line, src.Path)
		case src.Chart != "":
			line = joinNonEmpty(line, src.Chart)
		}
		if src.TargetRevision != "" {
			line = joinNonEmpty(line, src.TargetRevision)
		}
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "; ")
}

func argoDestination(obj map[string]any, fields ...string) string {
	name := nestedString(obj, append(fields, "name")...)
	server := nestedString(obj, append(fields, "server")...)
	ns := nestedString(obj, append(fields, "namespace")...)
	target := name
	if target == "" {
		target = server
	}
	if ns == "" {
		return target
	}
	if target == "" {
		return ns
	}
	return target + "/" + ns
}

func argoPolicy(obj map[string]any) string {
	auto, found, _ := unstructured.NestedMap(obj, "spec", "syncPolicy", "automated")
	if !found {
		return "Manual"
	}
	parts := []string{"Automated"}
	if nestedBool(auto, "prune") {
		parts = append(parts, "prune")
	}
	if nestedBool(auto, "selfHeal") {
		parts = append(parts, "self-heal")
	}
	return strings.Join(parts, ", ")
}

func argoResources(obj map[string]any) []ArgoResource {
	items, _, _ := unstructured.NestedSlice(obj, "status", "resources")
	out := make([]ArgoResource, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ArgoResource{
			Kind:      nestedString(m, "kind"),
			Namespace: nestedString(m, "namespace"),
			Name:      nestedString(m, "name"),
			Sync:      nestedString(m, "status"),
			Health:    nestedString(m, "health", "status"),
			Hook:      nestedBool(m, "hook"),
		})
	}
	return out
}

func argoHistory(obj map[string]any) []ArgoHistory {
	items, _, _ := unstructured.NestedSlice(obj, "status", "history")
	out := make([]ArgoHistory, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ArgoHistory{
			ID:       nestedInt(m, "id"),
			Revision: nestedString(m, "revision"),
			By:       argoInitiatedBy(m, "initiatedBy"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func argoConditions(obj map[string]any, fields ...string) []string {
	items, _, _ := unstructured.NestedSlice(obj, fields...)
	out := make([]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind := nestedString(m, "type")
		msg := nestedString(m, "message")
		if kind == "" && msg == "" {
			continue
		}
		if msg == "" {
			out = append(out, kind)
			continue
		}
		if kind == "" {
			out = append(out, msg)
			continue
		}
		out = append(out, kind+": "+msg)
	}
	return out
}

func argoGeneratorTypes(obj map[string]any) string {
	items, _, _ := unstructured.NestedSlice(obj, "spec", "generators")
	var names []string
	seen := map[string]bool{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if seen[key] {
				continue
			}
			seen[key] = true
			names = append(names, key)
		}
	}
	return strings.Join(names, ", ")
}

func argoProjectDestinations(obj map[string]any) []string {
	items, _, _ := unstructured.NestedSlice(obj, "spec", "destinations")
	out := make([]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		line := argoDestination(m)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func argoInitiatedBy(obj map[string]any, fields ...string) string {
	base := fields
	if nestedBool(obj, append(base, "automated")...) {
		user := nestedString(obj, append(base, "username")...)
		if user == "" {
			return "automated"
		}
		return "automated, " + user
	}
	return nestedString(obj, append(base, "username")...)
}

func decodeSecret(obj *unstructured.Unstructured, key string) string {
	raw := nestedString(obj.Object, "data", key)
	if raw == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return ""
	}
	return string(b)
}

func nestedString(obj map[string]any, fields ...string) string {
	v, found, err := unstructured.NestedString(obj, fields...)
	if err != nil || !found {
		return ""
	}
	return v
}

func nestedStrings(obj map[string]any, fields ...string) []string {
	items, _, _ := unstructured.NestedStringSlice(obj, fields...)
	return items
}

func nestedBool(obj map[string]any, fields ...string) bool {
	v, found, err := unstructured.NestedBool(obj, fields...)
	return err == nil && found && v
}

func nestedInt(obj map[string]any, fields ...string) int64 {
	v, found, err := unstructured.NestedInt64(obj, fields...)
	if err == nil && found {
		return v
	}
	raw, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !found {
		return 0
	}
	switch n := raw.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}
