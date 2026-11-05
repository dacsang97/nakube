package desktop

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// Appearance is how the app chooses its colors. System follows the computer.
type Appearance = mygo.ThemeSource

// DefaultAppearance follows the computer. Light and Dark are overrides.
func DefaultAppearance() Appearance { return mygo.ThemeSystem }

func parseAppearance(s string) (Appearance, bool) {
	switch Appearance(strings.TrimSpace(s)) {
	case mygo.ThemeLight, mygo.ThemeDark, mygo.ThemeSystem:
		return Appearance(strings.TrimSpace(s)), true
	default:
		return mygo.ThemeSystem, false
	}
}

func appearancePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nakube", "appearance"), nil
}

func readAppearance(path string) Appearance {
	body, err := os.ReadFile(path)
	if err != nil {
		return mygo.ThemeSystem
	}
	source, ok := parseAppearance(string(body))
	if !ok {
		return mygo.ThemeSystem
	}
	return source
}

func writeAppearance(path string, source Appearance) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(string(source)+"\n"), 0o600)
}

// RestoreAppearance applies a saved choice before the window opens. An
// absent file follows the computer.
func (a *App) RestoreAppearance() {
	path, err := appearancePath()
	if err != nil {
		return
	}
	a.applyAppearance(readAppearance(path), false)
}

func (a *App) applyAppearance(source Appearance, persist bool) {
	source, _ = parseAppearance(string(source))
	a.mu.Lock()
	a.appearance = source
	a.mu.Unlock()
	mygo.Theme.SetSource(source)
	if !persist {
		return
	}
	path, err := appearancePath()
	if err != nil {
		return
	}
	_ = writeAppearance(path, source)
}
