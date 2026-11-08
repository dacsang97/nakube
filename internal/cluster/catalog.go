package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// DefaultKubeDir is the user's ~/.kube, the folder the catalog syncs.
func DefaultKubeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube"), nil
}

// SyncKubeDir adds kubeconfig files found in dir to the loading list.
// A file already loaded is left as it is. Invalid files are skipped.
// It does not write into dir. An absent directory adds nothing.
func (s *Session) SyncKubeDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.New("kubeconfig could not be read")
	}
	if !info.IsDir() {
		return s.AddKubeconfigPath(dir)
	}
	files, err := kubeconfigFiles(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := s.AddKubeconfigPath(file); err != nil {
			return err
		}
	}
	return nil
}

// AddKubeconfigPath adds a file or a folder of kubeconfigs to the loading
// list. It does not write those clusters into ~/.kube.
func (s *Session) AddKubeconfigPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("kubeconfig path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return errors.New("kubeconfig could not be read")
	}
	var files []string
	if info.IsDir() {
		files, err = kubeconfigFiles(path)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return errors.New("folder has no kubeconfig")
		}
	} else {
		if _, err := loadKubeconfigFile(path); err != nil {
			return err
		}
		files = []string{path}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, file := range files {
		if !contains(s.extra, file) {
			s.extra = append(s.extra, file)
		}
	}
	s.rebuildRulesLocked()
	return nil
}

// ClearKubeconfigPaths removes every additional file while preserving the
// standard client-go loading rules.
func (s *Session) ClearKubeconfigPaths() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extra = nil
	s.rebuildRulesLocked()
}

// RemoveKubeconfigPath removes a previously added file or every added file
// beneath a folder, then rebuilds the loading precedence.
func (s *Session) RemoveKubeconfigPath(path string) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.extra[:0]
	for _, file := range s.extra {
		if samePath(file, path) || pathContains(path, file) {
			continue
		}
		kept = append(kept, file)
	}
	s.extra = kept
	s.rebuildRulesLocked()
}

func pathContains(parent, child string) bool {
	parent, errParent := filepath.Abs(parent)
	child, errChild := filepath.Abs(child)
	if errParent != nil || errChild != nil {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ExtraKubeconfigs lists files added beside the default ~/.kube search.
func (s *Session) ExtraKubeconfigs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.extra...)
}

// ImportKubeconfig writes body to dir and adds that file. dir must not be
// the user's ~/.kube. The error never includes the file body.
func (s *Session) ImportKubeconfig(dir, body string) (string, error) {
	cfg, err := clientcmd.Load([]byte(body))
	if err != nil || cfg == nil || len(cfg.Contexts) == 0 {
		return "", errors.New("kubeconfig has no contexts")
	}
	homeKube, err := DefaultKubeDir()
	if err == nil && samePath(dir, homeKube) {
		return "", errors.New("pasted kubeconfig is not written into ~/.kube")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", errors.New("kubeconfig could not be saved")
	}
	sum := sha256.Sum256([]byte(body))
	name := hex.EncodeToString(sum[:4]) + ".yaml"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return "", errors.New("kubeconfig could not be saved")
	}
	if err := s.AddKubeconfigPath(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Session) rebuildRulesLocked() {
	base := clientcmd.NewDefaultClientConfigLoadingRules()
	seen := map[string]bool{}
	var all []string
	for _, path := range append(base.GetLoadingPrecedence(), s.extra...) {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		all = append(all, path)
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = ""
	rules.Precedence = all
	s.rules = rules
}

func kubeconfigFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.New("kubeconfig could not be read")
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name != "config" && !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(dir, name)
		if _, err := loadKubeconfigFile(path); err != nil {
			continue
		}
		out = append(out, path)
	}
	return out, nil
}

func loadKubeconfigFile(path string) (*clientcmdapi.Config, error) {
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil || cfg == nil || len(cfg.Contexts) == 0 {
		return nil, fmt.Errorf("kubeconfig could not be read")
	}
	return cfg, nil
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return aa == bb
}
