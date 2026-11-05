package desktop

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The shell is two cards on a gutter. The activity bar and sidebar share
// one card. The editor is the other. Both sit inset from the window.
const (
	activityBarW   = 48
	activitySlot   = 40
	defaultSidebar = 240
	minSidebar     = 180
	maxSidebar     = 420
	sideTitleH     = 35
	sideRowH       = 28
	shellGap       = 4
	shellRadius    = 10
)

func shellBackdrop(t *ui.Theme) ui.Color {
	if t.Dark {
		return t.Background
	}
	return t.SurfaceHover
}

func editorCard(t *ui.Theme) ui.Color {
	if t.Dark {
		return t.Surface
	}
	return t.Background
}

func (a *App) sideWidth() float32 {
	if a.sidebarWidth < minSidebar {
		return defaultSidebar
	}
	if a.sidebarWidth > maxSidebar {
		return maxSidebar
	}
	return a.sidebarWidth
}

func (a *App) drawActivityBar(c *ui.Context, t *ui.Theme, current string) {
	a.ensureCatalog()
	page := ""
	if a.router != nil {
		page = screenID(a.router.Path())
	}
	ui.Column(c).Width(activityBarW).FillHeight().Shrink(0).Background(t.Surface).BorderEndWidth(1).BorderColor(t.Border).Children(func() {
		ui.Scroll(c).Grow(1).FillHeight().Children(func() {
			ui.Column(c).Padding(4, 0).Gap(4).AlignItems(ui.Center).Children(func() {
				for _, name := range a.catalogPins {
					a.pinButton(c, t, name, name == current)
				}
				a.activityButton(c, t, "plus", "Catalog", page == "catalog", func() {
					a.togglePage("/catalog")
				})
			})
		})
		ui.Column(c).Shrink(0).Padding(4, 0, 8, 0).AlignItems(ui.Center).Children(func() {
			a.activityButton(c, t, "settings", "Settings", page == "settings", func() {
				a.togglePage("/settings")
			})
		})
	})
}

func (a *App) activityButton(c *ui.Context, t *ui.Theme, icon, tip string, active bool, fn func()) {
	slot := ui.Box(c).Height(activitySlot).AlignSelf(ui.Stretch).AlignItems(ui.Center).Justify(ui.Center)
	slot.Children(func() {
		if active {
			ui.Box(c).Absolute().Left(0).Top(9).Width(2).Height(22).Radius(1).Background(t.Accent)
		}
		btn := ui.Button(c, "").Size(32, 32).Padding(0).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Tooltip(tip)
		if active {
			btn.TextColor(t.Text).Background(t.SurfacePressed)
		} else {
			btn.TextColor(t.TextMuted)
		}
		if btn.Hovered() {
			btn.Background(t.SurfaceHover)
		}
		btn.Children(func() {
			ui.Icon(c, IconNamed(icon)).Size(20, 20)
		})
		if btn.Clicked() {
			fn()
		}
	})
}

func (a *App) pinButton(c *ui.Context, t *ui.Theme, name string, active bool) {
	ui.Box(c).Height(activitySlot).AlignSelf(ui.Stretch).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		btn := ui.Button(c, "").Size(32, 32).Padding(0).Radius(8).AlignItems(ui.Center).Justify(ui.Center).Tooltip(name).Label(name)
		btn.Border(0, ui.Transparent)
		if active {
			btn.Background(t.Accent).TextColor(t.AccentText)
		} else {
			btn.Background(t.SurfacePressed).TextColor(t.Text)
		}
		btn.Children(func() {
			ui.Text(c, railLabel(name)).FontSize(13).FontWeight(650)
		})
		if btn.Clicked() {
			a.switchContext(name)
			a.openPage("overview")
		}
	})
}

func (a *App) drawSide(c *ui.Context, t *ui.Theme) {
	page := ""
	if a.router != nil {
		page = screenID(a.router.Path())
	}
	ui.Column(c).Width(a.sideWidth()).Shrink(0).FillHeight().Background(t.Surface).Children(func() {
		ui.Row(c).Height(sideTitleH).Shrink(0).Padding(0, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "MENU").FontSize(11).FontWeight(600).TextColor(t.TextMuted).SingleLine()
		})
		ui.Column(c).Shrink(0).Padding(8, 8, 4, 8).Children(func() {
			ui.TextInput(c, &a.menuQuery).Placeholder("Search").Grow(1)
		})
		ui.Scroll(c).Grow(1).FillHeight().Children(func() {
			ui.Column(c).Padding(4, 8, 8, 8).Gap(2).Children(func() {
				shown := 0
				for _, section := range Sections() {
					items := menuItems(section, a.menuQuery)
					if len(items) == 0 {
						continue
					}
					shown++
					ui.Text(c, strings.ToUpper(section.Title)).FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(8, 8, 2, 8)
					for _, item := range items {
						a.sideRow(c, t, item, item.ID == page)
					}
				}
				if shown == 0 {
					ui.Text(c, "No matches").TextColor(t.TextMuted).Padding(8, 8)
				}
			})
		})
		a.sideResize(c)
	})
}

func menuItems(section Section, query string) []Item {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return section.Items
	}
	if strings.Contains(strings.ToLower(section.Title), q) {
		return section.Items
	}
	var items []Item
	for _, item := range section.Items {
		if strings.Contains(strings.ToLower(item.Label), q) || strings.Contains(strings.ToLower(item.ID), q) {
			items = append(items, item)
		}
	}
	return items
}

func (a *App) sideRow(c *ui.Context, t *ui.Theme, item Item, selected bool) {
	row := ui.Row(c).Height(sideRowH).Padding(0, 8).Gap(8).Radius(4).AlignItems(ui.Center)
	icon := t.TextMuted
	if selected {
		row.Background(t.SurfacePressed)
		icon = t.Accent
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Icon(c, IconFor(item.ID)).Size(16, 16).TextColor(icon).Shrink(0)
		label := ui.Text(c, item.Label).FontSize(13).SingleLine().MinWidth(0).Grow(1)
		if selected {
			label.TextColor(t.Text)
		} else {
			label.TextColor(t.Text)
		}
	})
	if row.Clicked() {
		a.openPage(item.ID)
	}
}

// sideResize is the sidebar's right edge. A drag keeps the width for this
// window; a double click returns the default.
func (a *App) sideResize(c *ui.Context) {
	h := ui.Box(c.Key("side-resize")).Absolute().Right(-3).Top(0).Bottom(0).Width(6).Cursor(ui.CursorResizeColumn)
	if dx, _, ok := h.Dragged(); ok {
		if a.sideDrag == 0 {
			a.sideDrag = a.sideWidth()
		}
		a.sideDrag += dx
		if a.sideDrag < minSidebar {
			a.sideDrag = minSidebar
		}
		if a.sideDrag > maxSidebar {
			a.sideDrag = maxSidebar
		}
		a.sidebarWidth = a.sideDrag
		return
	}
	if a.sideDrag != 0 && !h.Pressed() {
		a.sideDrag = 0
	}
	if h.DoubleClicked() {
		a.sidebarWidth = defaultSidebar
	}
}
