package desktop

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// drawSettings is a page of this window. It is not a second window.
func (a *App) drawSettings(c *ui.Context, t *ui.Theme) {
	index := appearanceIndex(a.appearance)
	ui.Scroll(c).Grow(1).FillHeight().Children(func() {
		ui.Column(c).MaxWidth(520).Gap(16).Children(func() {
			ui.Text(c, "Settings").FontSize(22).Bold()
			ui.Text(c, "Preferences for this window.").TextColor(t.TextMuted)
			ui.Column(c).Padding(16).Gap(14).Radius(12).Background(t.Surface).Border(1, t.Border).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, "Appearance").FontSize(16).Bold()
						ui.Text(c, "System follows this computer. Light and Dark apply to the app.").TextColor(t.TextMuted)
					})
					ui.Segmented(c, &index, "System", "Light", "Dark").Shrink(0).Label("Appearance")
				})
				ui.Row(c).Gap(12).Children(func() {
					a.appearancePreview(c, t, "Light", ui.LightTheme(), a.appearance == mygo.ThemeLight)
					a.appearancePreview(c, t, "Dark", ui.DarkTheme(), a.appearance == mygo.ThemeDark)
				})
			})
		})
	})
	next := appearanceFromIndex(index)
	if next != a.appearance {
		a.applyAppearance(next, true)
	}
}

func (a *App) appearancePreview(c *ui.Context, t *ui.Theme, name string, sample *ui.Theme, chosen bool) {
	border := t.Border
	if chosen {
		border = t.Accent
	}
	ui.Themed(c, sample).Grow(1).MinWidth(0).Padding(12).Gap(8).Radius(10).Border(1, border).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, name).Bold().Grow(1)
			if chosen {
				ui.Text(c, "On").TextColor(sample.Accent)
			}
		})
		ui.Text(c, "Aa").FontSize(22).Bold()
		ui.Button(c, "Button")
	})
}

func appearanceIndex(source Appearance) int {
	switch source {
	case mygo.ThemeLight:
		return 1
	case mygo.ThemeDark:
		return 2
	default:
		return 0
	}
}

func appearanceFromIndex(index int) Appearance {
	switch index {
	case 1:
		return mygo.ThemeLight
	case 2:
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}
