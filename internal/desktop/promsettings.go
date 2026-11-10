package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/dacsang97/nakube/internal/cluster"
)

// promOverridesPath holds a Prometheus service per kubeconfig context.
func promOverridesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nakube", "prometheus.json"), nil
}

// readPromOverrides drops an entry that does not parse, so a hand edit
// cannot point the proxy at a path of its own.
func readPromOverrides(path string) map[string]string {
	out := map[string]string{}
	body, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var raw map[string]string
	if json.Unmarshal(body, &raw) != nil {
		return out
	}
	for name, value := range raw {
		if src, err := cluster.ParsePromSource(value); err == nil {
			out[name] = src.String()
		}
	}
	return out
}

func writePromOverrides(path string, overrides map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(overrides, "", "  ")
	if err != nil {
		return err
	}
	// A temp file renamed into place: a crash cannot leave half a file,
	// and an older file with wider permissions is replaced, not kept.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".prometheus-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// promOverridesLocked reads the file once, on first use.
func (a *App) promOverridesLocked() map[string]string {
	if !a.promLoaded {
		a.promLoaded = true
		if path, err := promOverridesPath(); err == nil {
			a.promOverrides = readPromOverrides(path)
		}
	}
	return a.promOverrides
}

// savePromOverride stores draft for the open context; empty removes it.
// It returns the message to show, or "" when it was saved.
func (a *App) savePromOverride(contextName, draft string) string {
	draft = strings.TrimSpace(draft)
	value := ""
	if draft != "" {
		src, err := cluster.ParsePromSource(draft)
		if err != nil {
			return "Use namespace/service:port, as monitoring/prometheus-operated:9090."
		}
		value = src.String()
	}
	path, err := promOverridesPath()
	if err != nil {
		return "Settings folder unavailable."
	}
	a.mu.Lock()
	current := a.promOverridesLocked()
	next := make(map[string]string, len(current)+1)
	for k, v := range current {
		next[k] = v
	}
	if value == "" {
		delete(next, contextName)
	} else {
		next[contextName] = value
	}
	a.mu.Unlock()
	if err := writePromOverrides(path, next); err != nil {
		return "Could not save: " + err.Error()
	}
	a.mu.Lock()
	a.promOverrides = next
	a.forgetMetricsLocked()
	a.mu.Unlock()
	a.invalidate()
	return ""
}
