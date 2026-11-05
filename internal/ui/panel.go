package ui

import "github.com/egoist/mygo/ui"

// Panel is a calm card: large radius, a hairline border, a fill that sits
// off the page in both themes.
func Panel(c *ui.Context, t *ui.Theme) ui.Element {
	fill := t.Surface
	if t.Dark {
		fill = t.SurfaceHover
	}
	return ui.Column(c).MinWidth(0).Padding(16).Gap(10).Radius(16).Background(fill).Border(1, t.Border)
}

// Title paints a colored section name with an optional icon tile.
func Title(c *ui.Context, t *ui.Theme, icon *ui.SVG, text string, color ui.Color) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if icon != nil {
			ui.Box(c).Size(26, 26).Radius(8).Shrink(0).Background(Tint(t, color)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Icon(c, icon).Size(14, 14).TextColor(color)
			})
		}
		ui.Text(c, text).Bold().TextColor(color)
	})
}
