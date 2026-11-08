package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// clientQPS matches a desktop that watches several kinds at once.
	clientQPS   = 50
	clientBurst = 100
)

// ErrNoContext is returned when a call needs an open context and none is.
var ErrNoContext = errors.New("no kube context is open")

// ErrNoKubeconfig is returned when no context can be selected.
var ErrNoKubeconfig = errors.New("no kube context is configured")

// Session is the only Kubernetes client. The window, MCP, and phone are
// adapters over it. It does not import a UI toolkit and does not listen.
type Session struct {
	mu sync.Mutex

	rules     *clientcmd.ClientConfigLoadingRules
	extra     []string
	statePath string

	name      string
	namespace string
	cfg       *rest.Config
	cs        kubernetes.Interface
	dyn       dynamic.Interface
	mapper    meta.RESTMapper

	watchGen    uint64
	watchCancel context.CancelFunc
	factory     any
	watchSync   []namedSync
}

// NewSession returns a session that has not opened a context yet.
func NewSession() *Session {
	return &Session{}
}

// StatePath is the file Use writes and the MCP process reads.
func StatePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nakube", "context"), nil
}

// SetStatePath overrides the file Use writes. Tests call it so a switch in
// a test session never touches the real per-user state file.
func (s *Session) SetStatePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statePath = path
}

// ReadSelectedContext reads the context name Use wrote. ok is false when the
// file is absent or empty. The error never includes file contents.
func ReadSelectedContext(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	name := strings.TrimSpace(string(b))
	if name == "" {
		return "", false, nil
	}
	if strings.ContainsAny(name, "\r\n") || strings.Contains(name, "\x00") {
		return "", false, errors.New("context state file is invalid")
	}
	return name, true, nil
}

// RefreshLoadingRules re-reads kubeconfig search precedence. Call it after
// ImportShellEnv, because KUBECONFIG may exist only in the login shell.
// Files already added stay on the list.
func (s *Session) RefreshLoadingRules() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rebuildRulesLocked()
}

// Open selects the context in the state file, or kubeconfig current-context
// when that file is absent. It does not start watches and does not wait for
// a shell import.
func (s *Session) Open(ctx context.Context) error {
	s.RefreshLoadingRules()
	path, err := s.stateFile()
	if err != nil {
		return err
	}
	name, ok, err := ReadSelectedContext(path)
	if err != nil {
		return err
	}
	if !ok {
		_, current, err := s.Contexts()
		if err != nil {
			return err
		}
		name = current
	}
	if name == "" {
		return ErrNoKubeconfig
	}
	return s.Use(ctx, name)
}

// Use replaces the in-memory context, stops previous watches, and writes the
// selected name for the MCP process. There is no aggregated context list.
func (s *Session) Use(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if name == "" {
		return errors.New("context name is required")
	}
	s.mu.Lock()
	s.stopWatchLocked()
	rules := s.rules
	s.mu.Unlock()
	if rules == nil {
		s.RefreshLoadingRules()
	}
	return s.openContext(ctx, name)
}

// Contexts lists kubeconfig context names. An empty file returns an empty
// list. The error does not include the file body.
func (s *Session) Contexts() ([]string, string, error) {
	raw, err := s.rawConfig()
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(raw.Contexts))
	for name := range raw.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, raw.CurrentContext, nil
}

// ContextInfo is one catalog line. Server has no user, query, or fragment,
// so a token in the URL is not shown.
type ContextInfo struct {
	Name      string
	Cluster   string
	Namespace string
	Server    string
}

// ContextInfos lists contexts for the catalog. It does not include users,
// tokens, certificates, or exec commands.
func (s *Session) ContextInfos() ([]ContextInfo, string, error) {
	raw, err := s.rawConfig()
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(raw.Contexts))
	for name := range raw.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ContextInfo, 0, len(names))
	for _, name := range names {
		info := ContextInfo{Name: name}
		ctx := raw.Contexts[name]
		if ctx != nil {
			info.Cluster = ctx.Cluster
			info.Namespace = ctx.Namespace
			if cl := raw.Clusters[ctx.Cluster]; cl != nil {
				info.Server = publicServer(cl.Server)
			}
		}
		out = append(out, info)
	}
	return out, raw.CurrentContext, nil
}

func publicServer(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Scheme == "" {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// Current is the in-memory context name, or "" when nothing is open.
func (s *Session) Current() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// Namespace is the kubeconfig default namespace for the open context.
func (s *Session) Namespace() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.namespace
}

// RESTConfig returns a copy of the open context's config. Callers must not
// log it: it can carry a bearer token.
func (s *Session) RESTConfig() (*rest.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		return nil, ErrNoContext
	}
	return rest.CopyConfig(s.cfg), nil
}

func (s *Session) clientset() (kubernetes.Interface, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cs == nil {
		return nil, ErrNoContext
	}
	return s.cs, nil
}

// Clientset is the open context's client. Adapters must not build another one.
func (s *Session) Clientset() (kubernetes.Interface, error) {
	return s.clientset()
}

func (s *Session) dynamicClient() (dynamic.Interface, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dyn == nil {
		return nil, ErrNoContext
	}
	return s.dyn, nil
}

func (s *Session) stateFile() (string, error) {
	s.mu.Lock()
	path := s.statePath
	s.mu.Unlock()
	if path != "" {
		return path, nil
	}
	return StatePath()
}

func (s *Session) rawConfig() (*clientcmdapi.Config, error) {
	s.mu.Lock()
	if s.rules == nil {
		s.rules = clientcmd.NewDefaultClientConfigLoadingRules()
	}
	rules := s.rules
	s.mu.Unlock()
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	raw, err := cc.RawConfig()
	if err != nil {
		return nil, errors.New("kubeconfig could not be read")
	}
	return &raw, nil
}

func (s *Session) openContext(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.rules == nil {
		s.rules = clientcmd.NewDefaultClientConfigLoadingRules()
	}
	rules := s.rules
	s.mu.Unlock()

	rawCC := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	raw, err := rawCC.RawConfig()
	if err != nil {
		return errors.New("kubeconfig could not be read")
	}
	if _, ok := raw.Contexts[name]; !ok {
		return fmt.Errorf("kube context %q is not in the kubeconfig", name)
	}
	ns := ""
	if c := raw.Contexts[name]; c != nil {
		ns = c.Namespace
	}

	overrides := &clientcmd.ConfigOverrides{CurrentContext: name}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return errors.New("kube context could not be loaded")
	}
	cfg.QPS = clientQPS
	cfg.Burst = clientBurst

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.New("kubernetes client could not be built")
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return errors.New("dynamic client could not be built")
	}
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return errors.New("discovery client could not be built")
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	s.mu.Lock()
	s.stopWatchLocked()
	s.name = name
	s.namespace = ns
	s.cfg = cfg
	s.cs = cs
	s.dyn = dyn
	s.mapper = mapper
	s.mu.Unlock()
	return s.writeState(name)
}

func (s *Session) writeState(name string) error {
	path, err := s.stateFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(name+"\n"), 0o600)
}

func (s *Session) stopWatchLocked() {
	s.watchGen++
	if s.watchCancel != nil {
		s.watchCancel()
		s.watchCancel = nil
	}
	s.factory = nil
	s.watchSync = nil
}

type namedSync struct {
	kind string
	inf  interface{ HasSynced() bool }
}

// Synced reports whether pods, nodes, and events have finished their first
// list. Other informers may still be listing. It is false before Watch.
func (s *Session) Synced() bool {
	s.mu.Lock()
	syncs := append([]namedSync(nil), s.watchSync...)
	s.mu.Unlock()
	need := map[string]bool{"Pod": false, "Node": false, "Event": false}
	for _, item := range syncs {
		if _, ok := need[item.kind]; !ok {
			continue
		}
		if item.inf == nil || !item.inf.HasSynced() {
			return false
		}
		need[item.kind] = true
	}
	for _, ok := range need {
		if !ok {
			return false
		}
	}
	return true
}

// KindSynced reports whether that informer's first list has finished.
// An unknown kind, or a watch that has not started, is false.
func (s *Session) KindSynced(kind string) bool {
	s.mu.Lock()
	syncs := append([]namedSync(nil), s.watchSync...)
	s.mu.Unlock()
	for _, item := range syncs {
		if item.kind != kind {
			continue
		}
		return item.inf != nil && item.inf.HasSynced()
	}
	return false
}
