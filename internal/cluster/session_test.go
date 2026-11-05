package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const twoContextKubeconfig = `
apiVersion: v1
kind: Config
current-context: a
contexts:
- name: a
  context:
    cluster: c
    user: u
- name: b
  context:
    cluster: c
    user: u
    namespace: team
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:6443
users:
- name: u
  user:
    token: test-token-not-for-logs
`

func TestUseWritesStateAndStopsWatch(t *testing.T) {
	dir := t.TempDir()
	kubeconfig := filepath.Join(dir, "config")
	if err := os.WriteFile(kubeconfig, []byte(twoContextKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)

	s := NewSession()
	s.statePath = filepath.Join(dir, "state", "context")
	s.RefreshLoadingRules()
	if err := s.Use(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	if s.Current() != "b" {
		t.Fatalf("current = %q", s.Current())
	}
	if s.Namespace() != "team" {
		t.Fatalf("namespace = %q", s.Namespace())
	}
	name, ok, err := ReadSelectedContext(s.statePath)
	if err != nil || !ok || name != "b" {
		t.Fatalf("state %q ok=%v err=%v", name, ok, err)
	}
	body, err := os.ReadFile(s.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "test-token") {
		t.Fatalf("state file contains a token: %q", body)
	}

	cancelled := false
	s.mu.Lock()
	s.watchCancel = func() { cancelled = true }
	s.mu.Unlock()
	if err := s.Use(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("previous watch was not stopped")
	}
	if s.Current() != "a" {
		t.Fatalf("current = %q", s.Current())
	}
}

func TestOpenPrefersStateFile(t *testing.T) {
	dir := t.TempDir()
	kubeconfig := filepath.Join(dir, "config")
	if err := os.WriteFile(kubeconfig, []byte(twoContextKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)
	s := NewSession()
	s.statePath = filepath.Join(dir, "context")
	if err := os.WriteFile(s.statePath, []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.Current() != "b" {
		t.Fatalf("current = %q, want state file context", s.Current())
	}
}

func TestOpenEmptyKubeconfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	s := NewSession()
	s.statePath = filepath.Join(dir, "context")

	err := s.Open(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "apiVersion") {
		t.Fatalf("error includes kubeconfig material: %v", err)
	}
	names, current, err := s.Contexts()
	if err != nil {
		t.Fatal(err)
	}
	if current != "" || len(names) != 0 {
		t.Fatalf("names=%v current=%q", names, current)
	}
}

func TestReadSelectedContextMissing(t *testing.T) {
	name, ok, err := ReadSelectedContext(filepath.Join(t.TempDir(), "missing"))
	if err != nil || ok || name != "" {
		t.Fatalf("name=%q ok=%v err=%v", name, ok, err)
	}
}

func TestSyncedBeforeWatch(t *testing.T) {
	if NewSession().Synced() {
		t.Fatal("synced before a watch")
	}
}
