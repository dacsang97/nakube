package cluster

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPreparePathPrependsExistingDirectoriesOnce(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin")

	PreparePath()

	parts := filepath.SplitList(os.Getenv("PATH"))
	if parts[len(parts)-1] != "/usr/bin" {
		t.Fatalf("original PATH should stay the suffix, got %q", os.Getenv("PATH"))
	}
	if !slices.Contains(parts, binDir) {
		t.Fatalf("expected %q in PATH, got %q", binDir, os.Getenv("PATH"))
	}

	PreparePath()
	count := 0
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == binDir {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected %q once after a second call, got %d", binDir, count)
	}
}

func TestPreparePathSkipsMissingDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	PreparePath()

	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if !strings.Contains(p, home) {
			continue
		}
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			t.Fatalf("missing directory %q ended up in PATH", p)
		}
	}
}
