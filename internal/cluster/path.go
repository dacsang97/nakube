package cluster

import (
	"os"
	"path/filepath"
	"strings"
)

// PreparePath prepends directories a Dock or Finder launch does not put on
// PATH, so exec credential plugins can be found. It is not an init function.
func PreparePath() {
	candidates := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "bin"),
			filepath.Join(home, ".rd", "bin"),
			filepath.Join(home, ".docker", "bin"),
			filepath.Join(home, "google-cloud-sdk", "bin"),
			filepath.Join(home, ".krew", "bin"),
			filepath.Join(home, ".local", "bin"),
		)
	}

	current := os.Getenv("PATH")
	present := make(map[string]bool)
	for _, dir := range filepath.SplitList(current) {
		present[dir] = true
	}

	additions := make([]string, 0, len(candidates))
	for _, dir := range candidates {
		if dir == "" || present[dir] {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		additions = append(additions, dir)
		present[dir] = true
	}
	if len(additions) == 0 {
		return
	}

	merged := strings.Join(additions, string(filepath.ListSeparator))
	if current != "" {
		merged += string(filepath.ListSeparator) + current
	}
	_ = os.Setenv("PATH", merged)
}
