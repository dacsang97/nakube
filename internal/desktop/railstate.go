package desktop

import (
	"os"
	"path/filepath"
	"strings"
)

func railPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nakube", "rail"), nil
}

// readRail reports whether the rail was left expanded. A missing or unknown
// file keeps the compact rail.
func readRail(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(body)) == "expanded"
}

func writeRail(path string, expanded bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	state := "collapsed"
	if expanded {
		state = "expanded"
	}
	return os.WriteFile(path, []byte(state+"\n"), 0o600)
}

// RestoreRail applies the saved rail width before the window opens.
func (a *App) RestoreRail() {
	path, err := railPath()
	if err != nil {
		return
	}
	a.railExpanded = readRail(path)
}

func (a *App) toggleRail() {
	a.railExpanded = !a.railExpanded
	path, err := railPath()
	if err != nil {
		return
	}
	_ = writeRail(path, a.railExpanded)
}
