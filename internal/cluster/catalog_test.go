package cluster

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pastedKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- name: pasted
  cluster:
    server: https://127.0.0.1:6443
contexts:
- name: pasted-ctx
  context:
    cluster: pasted
    user: pasted
current-context: pasted-ctx
users:
- name: pasted
  user:
    token: secret-token
`

func TestImportKubeconfigDoesNotTouchHomeKube(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	kube := filepath.Join(home, ".kube")
	if err := os.MkdirAll(kube, 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte("apiVersion: v1\nkind: Config\n")
	if err := os.WriteFile(filepath.Join(kube, "config"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	dir := t.TempDir()
	path, err := s.ImportKubeconfig(dir, pastedKubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(path, kube) {
		t.Fatalf("wrote into ~/.kube: %s", path)
	}
	got, err := os.ReadFile(filepath.Join(kube, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("~/.kube/config was rewritten")
	}
	names, _, err := s.Contexts()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(names, "pasted-ctx") {
		t.Fatalf("contexts = %v", names)
	}
}

func TestImportRejectsEmptyContexts(t *testing.T) {
	s := NewSession()
	_, err := s.ImportKubeconfig(t.TempDir(), "apiVersion: v1\nkind: Config\n")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err = %v", err)
	}
}

func TestAddFolderReadsConfigFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(pastedKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if err := s.AddKubeconfigPath(dir); err != nil {
		t.Fatal(err)
	}
	names, _, err := s.Contexts()
	if err != nil {
		t.Fatal(err)
	}
	if !contains(names, "pasted-ctx") {
		t.Fatalf("contexts = %v", names)
	}
}

func TestRemoveKubeconfigFolderDropsItsFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KUBECONFIG", filepath.Join(home, "missing"))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(pastedKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if err := s.AddKubeconfigPath(dir); err != nil {
		t.Fatal(err)
	}
	if len(s.ExtraKubeconfigs()) != 1 {
		t.Fatalf("extra kubeconfigs = %v", s.ExtraKubeconfigs())
	}
	s.RemoveKubeconfigPath(dir)
	if len(s.ExtraKubeconfigs()) != 0 {
		t.Fatalf("removed folder remains in %v", s.ExtraKubeconfigs())
	}
	infos, _, err := s.ContextInfos()
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		if info.Name == "pasted-ctx" {
			t.Fatalf("removed context remains in %+v", infos)
		}
	}
}

func TestContextInfoHidesCredentials(t *testing.T) {
	dir := t.TempDir()
	body := `
apiVersion: v1
kind: Config
clusters:
- name: secret-cluster
  cluster:
    server: https://user:secret-token@127.0.0.1:6443/k8s?token=secret-token#frag
contexts:
- name: hidden
  context:
    cluster: secret-cluster
    namespace: demo
    user: someone
current-context: hidden
`
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	if err := s.AddKubeconfigPath(path); err != nil {
		t.Fatal(err)
	}
	infos, current, err := s.ContextInfos()
	if err != nil {
		t.Fatal(err)
	}
	if current != "hidden" || len(infos) != 1 {
		t.Fatalf("current %q infos %+v", current, infos)
	}
	got := infos[0]
	if got.Name != "hidden" || got.Cluster != "secret-cluster" || got.Namespace != "demo" {
		t.Fatalf("%+v", got)
	}
	if got.Server != "https://127.0.0.1:6443/k8s" || strings.Contains(got.Server, "secret-token") {
		t.Fatalf("server %q", got.Server)
	}
	if publicServer("not a url") != "" || publicServer("https://token@example.test") != "https://example.test" {
		t.Fatal("publicServer leaked or kept a bad url")
	}
}

func TestSyncKubeDirPicksUpALaterFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".kube")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(pastedKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.yaml"), []byte("not a kubeconfig\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	s.RefreshLoadingRules()
	if err := s.SyncKubeDir(dir); err != nil {
		t.Fatal(err)
	}
	names, _, err := s.Contexts()
	if err != nil || !contains(names, "pasted-ctx") {
		t.Fatalf("names %v err %v", names, err)
	}
	later := strings.ReplaceAll(pastedKubeconfig, "pasted-ctx", "later-ctx")
	later = strings.ReplaceAll(later, "name: pasted", "name: later")
	if err := os.WriteFile(filepath.Join(dir, "later.yaml"), []byte(later), 0o600); err != nil {
		t.Fatal(err)
	}
	names, _, err = s.Contexts()
	if err != nil {
		t.Fatal(err)
	}
	if contains(names, "later-ctx") {
		t.Fatal("a new file was visible before another scan")
	}
	if err := s.SyncKubeDir(dir); err != nil {
		t.Fatal(err)
	}
	names, _, err = s.Contexts()
	if err != nil || !contains(names, "later-ctx") || !contains(names, "pasted-ctx") {
		t.Fatalf("names %v err %v", names, err)
	}
}

func TestOpenKeepsContextFromSyncedFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KUBECONFIG", "")
	dir := filepath.Join(home, ".kube")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	side := strings.ReplaceAll(pastedKubeconfig, "pasted", "side")
	if err := os.WriteFile(filepath.Join(dir, "side.yaml"), []byte(side), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSession()
	s.statePath = filepath.Join(home, "state")
	if err := s.SyncKubeDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := s.Use(context.Background(), "side-ctx"); err != nil {
		t.Fatal(err)
	}
	restarted := NewSession()
	restarted.statePath = s.statePath
	if err := restarted.SyncKubeDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if restarted.Current() != "side-ctx" {
		t.Fatalf("current = %q", restarted.Current())
	}
}
