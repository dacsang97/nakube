package cluster

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestForContextDoesNotSwitchDesktop(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	if err := os.WriteFile(config, []byte(twoContextKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", config)
	desktop := NewSession()
	state := filepath.Join(dir, "selected")
	desktop.SetStatePath(state)
	if err := desktop.Use(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	cancelled := false
	desktop.watchCancel = func() { cancelled = true }
	mobile, err := desktop.ForContext("b")
	if err != nil {
		t.Fatal(err)
	}
	if mobile.Current() != "b" || desktop.Current() != "a" || cancelled {
		t.Fatalf("mobile=%q desktop=%q cancelled=%v", mobile.Current(), desktop.Current(), cancelled)
	}
	selected, err := os.ReadFile(state)
	if err != nil || string(selected) != "a\n" {
		t.Fatalf("state=%q err=%v", selected, err)
	}
	if _, err := desktop.ForContext("missing"); err == nil {
		t.Fatal("accepted unknown context")
	}
}
