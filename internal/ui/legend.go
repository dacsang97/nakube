package ui

import "github.com/egoist/mygo/ui"

// Swatch is one legend row. Color is the series, not the resource accent.
type Swatch struct {
	Color ui.Color
	Name  string
	Value string
}

// Legend lists swatches in order. Name and value never drift apart.
func Legend(c *ui.Context, t *ui.Theme, items []Swatch) {
	ui.Column(c).Gap(6).AlignSelf(ui.Stretch).Children(func() {
		for _, item := range items {
			name, value, color := item.Name, item.Value, item.Color
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Shrink(0).Background(color)
				ui.Text(c, name).Grow(1).TextColor(t.TextMuted).SingleLine()
				ui.Text(c, value).Shrink(0).Bold().SingleLine()
			})
		}
	})
}
