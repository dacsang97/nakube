package desktop

import (
	"github.com/egoist/mygo/ui"

	"github.com/dacsang97/nakube/internal/views/settings"
)

// drawSettings is a page of this window. It is not a second window.
func (a *App) drawSettings(c *ui.Context, t *ui.Theme) {
	index := settings.Index(a.appearance)
	settings.Draw(c, t, &index, a.appearance)
	if next := settings.FromIndex(index); next != a.appearance {
		a.applyAppearance(next, true)
	}
}
