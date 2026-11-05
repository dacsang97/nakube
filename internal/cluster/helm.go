package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

// HelmDryRunResult is the rendered manifest and notes. It is not a diff.
type HelmDryRunResult struct {
	Manifest string
	Notes    string
}

// HelmRequest is one install or upgrade. Confirm reapplies this same chart,
// values, and namespace. Cancel does not call the mutating action.
type HelmRequest struct {
	Namespace       string
	Release         string
	Chart           string
	Version         string
	Values          string
	CreateNamespace bool
	Wait            bool
	Atomic          bool
	TimeoutSeconds  int
	DryRun          bool
	ResetValues     bool
}

// helmMutate is the install/upgrade write. Dry-run does not call it.
// Tests set it to fail if a dry-run invokes a write.
var helmMutate func(req HelmRequest) error

// HelmInstall installs a chart. Dry-run returns the rendered manifest and
// does not call the mutating install.
func (s *Session) HelmInstall(ctx context.Context, req HelmRequest) (*HelmDryRunResult, error) {
	return s.helmApply(ctx, req, false)
}

// HelmUpgrade upgrades a release. Dry-run does not call the mutating upgrade.
func (s *Session) HelmUpgrade(ctx context.Context, req HelmRequest) (*HelmDryRunResult, error) {
	return s.helmApply(ctx, req, true)
}

func (s *Session) helmApply(ctx context.Context, req HelmRequest, upgrade bool) (*HelmDryRunResult, error) {
	ch, err := loadChart(req.Chart)
	if err != nil {
		return nil, err
	}
	if !upgrade {
		if err := validateChartInstallable(ch); err != nil {
			return nil, err
		}
	}
	vals, err := parseValues(req.Values)
	if err != nil {
		return nil, err
	}
	if req.DryRun {
		manifest, err := renderLocal(ch, req, vals)
		if err != nil {
			return nil, err
		}
		return &HelmDryRunResult{Manifest: manifest}, nil
	}
	if helmMutate != nil {
		return nil, helmMutate(req)
	}
	if upgrade {
		return nil, s.upgradeRelease(ctx, req, ch, vals)
	}
	return nil, s.installRelease(ctx, req, ch, vals)
}

func (s *Session) installRelease(ctx context.Context, req HelmRequest, ch *chart.Chart, vals map[string]any) error {
	cfg, err := s.helmConfig(req.Namespace)
	if err != nil {
		return err
	}
	inst := action.NewInstall(cfg)
	inst.ReleaseName = req.Release
	inst.Namespace = req.Namespace
	inst.CreateNamespace = req.CreateNamespace
	inst.Version = req.Version
	inst.Wait = req.Wait
	inst.Atomic = req.Atomic
	inst.DryRun = false
	inst.Timeout = helmTimeout(req)
	_, err = inst.RunWithContext(ctx, ch, vals)
	return err
}

func (s *Session) upgradeRelease(ctx context.Context, req HelmRequest, ch *chart.Chart, vals map[string]any) error {
	cfg, err := s.helmConfig(req.Namespace)
	if err != nil {
		return err
	}
	up := action.NewUpgrade(cfg)
	up.Namespace = req.Namespace
	up.Version = req.Version
	up.Wait = req.Wait
	up.Atomic = req.Atomic
	up.DryRun = false
	up.ResetValues = req.ResetValues
	up.Timeout = helmTimeout(req)
	_, err = up.RunWithContext(ctx, req.Release, ch, vals)
	return err
}

func helmTimeout(req HelmRequest) time.Duration {
	if req.TimeoutSeconds > 0 {
		return time.Duration(req.TimeoutSeconds) * time.Second
	}
	if req.Wait || req.Atomic {
		return 5 * time.Minute
	}
	return 0
}

func (s *Session) helmConfig(namespace string) (*action.Configuration, error) {
	getter, err := s.HelmGetter(namespace)
	if err != nil {
		return nil, err
	}
	cfg := new(action.Configuration)
	if err := cfg.Init(getter, namespace, "secret", func(string, ...any) {}); err != nil {
		return nil, err
	}
	return cfg, nil
}

// HelmGetter reports the session's current context. It does not re-read the
// kubeconfig file, so Use("B") cannot install against context A.
func (s *Session) HelmGetter(namespace string) (*sessionGetter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil || s.name == "" {
		return nil, ErrNoContext
	}
	return &sessionGetter{cfg: rest.CopyConfig(s.cfg), name: s.name, namespace: namespace}, nil
}

type sessionGetter struct {
	cfg       *rest.Config
	name      string
	namespace string
}

func (g *sessionGetter) ToRESTConfig() (*rest.Config, error) {
	if g.cfg == nil {
		return nil, ErrNoContext
	}
	return rest.CopyConfig(g.cfg), nil
}

func (g *sessionGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	cfg, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

func (g *sessionGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

func (g *sessionGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	raw := clientcmdapi.Config{
		CurrentContext: g.name,
		Contexts: map[string]*clientcmdapi.Context{
			g.name: {Cluster: g.name, AuthInfo: g.name, Namespace: g.namespace},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			g.name: {Server: g.cfg.Host},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			g.name: {},
		},
	}
	return clientcmd.NewDefaultClientConfig(raw, &clientcmd.ConfigOverrides{})
}

func loadChart(ref string) (*chart.Chart, error) {
	if ref == "" {
		return nil, errors.New("chart is required")
	}
	path := ref
	if _, err := os.Stat(path); err != nil {
		alt := filepath.Join("testdata", "charts", ref)
		if _, err2 := os.Stat(alt); err2 == nil {
			path = alt
		}
	}
	ch, err := loader.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}
	return ch, nil
}

func validateChartInstallable(ch *chart.Chart) error {
	if ch == nil || ch.Metadata == nil {
		return errors.New("chart has no metadata")
	}
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	default:
		return fmt.Errorf("chart %q is type %q (only application charts are installable)", ch.Metadata.Name, ch.Metadata.Type)
	}
}

func parseValues(body string) (map[string]any, error) {
	vals := map[string]any{}
	if body == "" {
		return vals, nil
	}
	if err := yaml.Unmarshal([]byte(body), &vals); err != nil {
		return nil, errors.New("invalid values YAML")
	}
	if vals == nil {
		vals = map[string]any{}
	}
	return vals, nil
}

func renderLocal(ch *chart.Chart, req HelmRequest, vals map[string]any) (string, error) {
	inst := action.NewInstall(&action.Configuration{})
	inst.DryRun = true
	inst.ClientOnly = true
	inst.ReleaseName = req.Release
	if inst.ReleaseName == "" {
		inst.ReleaseName = "demo"
	}
	inst.Namespace = req.Namespace
	if inst.Namespace == "" {
		inst.Namespace = "default"
	}
	inst.Replace = true
	inst.ClientOnly = true
	rel, err := inst.Run(ch, vals)
	if err != nil {
		return "", err
	}
	return rel.Manifest, nil
}

// HelmRollback rolls a release back. It confirms in the UI before this call.
func (s *Session) HelmRollback(ctx context.Context, namespace, name string, revision int) error {
	if helmMutate != nil {
		return helmMutate(HelmRequest{Namespace: namespace, Release: name, Version: fmt.Sprint(revision)})
	}
	cfg, err := s.helmConfig(namespace)
	if err != nil {
		return err
	}
	rb := action.NewRollback(cfg)
	rb.Version = revision
	return rb.Run(name)
}

// HelmUninstall removes the release. It does not walk remaining objects itself.
func (s *Session) HelmUninstall(ctx context.Context, namespace, name string) error {
	if helmMutate != nil {
		return helmMutate(HelmRequest{Namespace: namespace, Release: name})
	}
	cfg, err := s.helmConfig(namespace)
	if err != nil {
		return err
	}
	_, err = action.NewUninstall(cfg).Run(name)
	return err
}

// HelmRelease is one installed release. It has no chart values.
type HelmRelease struct {
	Name      string
	Namespace string
	Chart     string
	Status    string
	Revision  int
}

// HelmList lists releases in every namespace. It does not install or upgrade.
func (s *Session) HelmList(ctx context.Context) ([]HelmRelease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ns := s.Namespace()
	if ns == "" {
		ns = "default"
	}
	cfg, err := s.helmConfig(ns)
	if err != nil {
		return nil, err
	}
	l := action.NewList(cfg)
	l.All = true
	l.AllNamespaces = true
	rels, err := l.Run()
	if err != nil {
		return nil, err
	}
	out := make([]HelmRelease, 0, len(rels))
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		item := HelmRelease{Name: rel.Name, Namespace: rel.Namespace, Revision: rel.Version}
		if rel.Chart != nil && rel.Chart.Metadata != nil {
			item.Chart = rel.Chart.Metadata.Name
			if rel.Chart.Metadata.Version != "" {
				item.Chart += "-" + rel.Chart.Metadata.Version
			}
		}
		if rel.Info != nil {
			item.Status = rel.Info.Status.String()
		}
		out = append(out, item)
	}
	return out, nil
}

// HelmRepoAdd adds a chart repository. It does not install a chart.
func HelmRepoAdd(name, url, file string) error {
	f := repo.NewFile()
	if _, err := os.Stat(file); err == nil {
		loaded, err := repo.LoadFile(file)
		if err != nil {
			return err
		}
		f = loaded
	}
	if f.Has(name) {
		return fmt.Errorf("repository %q already exists", name)
	}
	entry := &repo.Entry{Name: name, URL: url}
	f.Add(entry)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return f.WriteFile(file, 0o644)
}

// HelmRepoRemove removes a repository entry. It does not delete releases.
func HelmRepoRemove(name, file string) error {
	f, err := repo.LoadFile(file)
	if err != nil {
		return err
	}
	if !f.Remove(name) {
		return fmt.Errorf("repository %q not found", name)
	}
	return f.WriteFile(file, 0o644)
}

// HelmSearch finds charts in a loaded index. It does not install.
func HelmSearch(indexYAML, term string) ([]string, error) {
	idx, err := repo.LoadIndexFile(indexYAML) // may not exist; tests pass a path
	if err != nil {
		return nil, err
	}
	var names []string
	for name := range idx.Entries {
		if term == "" || containsFold(name, term) {
			names = append(names, name)
		}
	}
	return names, nil
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || len(sub) > 0 && containsLower(s, sub))
}

func containsLower(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexFold(s, sub) >= 0)
}

func indexFold(s, sub string) int {
	ls, lsub := []rune(s), []rune(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		ok := true
		for j := range lsub {
			a, b := ls[i+j], lsub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
