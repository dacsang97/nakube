package desktop

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type shellDetailPanel struct {
	title    string
	label    string
	copyText string
	onEdit   func()
	onRemove func()
	draw     func(*ui.Context, *ui.Theme)
}

// The activity bar and sidebar share the left shell card. The editor is the
// center card; selected resources add an aligned detail card on the right.
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

func rightPanel(c *ui.Context, t *ui.Theme, label string) ui.Element {
	return ui.Column(c).Width(360).MaxWidthPercent(38).Shrink(0).FillHeight().
		Radius(shellRadius).Clip().Background(editorCard(t)).Border(1, t.Border).
		Label(label)
}

func (a *App) offerDetailPanel(changed, valid bool, panel shellDetailPanel) {
	if changed {
		a.detailPanelOpen = true
	}
	if !valid {
		a.detailPanelOpen = false
		return
	}
	if a.detailPanelOpen {
		a.detailPanel = &panel
	}
}

func (a *App) drawDetailPanel(c *ui.Context, t *ui.Theme) {
	if a.detailPanel == nil {
		return
	}
	panel := a.detailPanel
	rightPanel(c, t, panel.label).Children(func() {
		ui.Row(c).Height(sideTitleH).Shrink(0).Padding(0, 8, 0, 12).Gap(4).AlignItems(ui.Center).Background(tabInactive(t)).Label("Detail panel header").Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, panel.title).FontSize(13).FontWeight(650).SingleLine().Shrink(1).MinWidth(0)
				copyKey := panel.label + "\x00" + panel.title + "\x00" + panel.copyText
				copyLabel, copyIcon := "Copy name", "copy"
				if a.detailCopiedKey == copyKey && c.Now().Before(a.detailCopiedUntil) {
					copyLabel, copyIcon = "Copied", "check"
				}
				copyButton := panelHeaderButton(c, t, copyLabel, copyIcon, false)
				if copyButton.Clicked() {
					c.WriteClipboard(panel.copyText)
					a.detailCopiedKey = copyKey
					a.detailCopiedUntil = c.Now().Add(2 * time.Second)
					c.After(2 * time.Second)
				}
			})
			if panel.onEdit != nil && panelHeaderButton(c, t, "Edit", "pencil", false).Clicked() {
				panel.onEdit()
			}
			if panel.onRemove != nil && panelHeaderButton(c, t, "Remove", "trash-2", true).Clicked() {
				panel.onRemove()
			}
			if panelHeaderButton(c, t, "Close details", "x", false).Clicked() {
				a.row = -1
				a.listKey = ""
				a.detailPanel = nil
				a.detailPanelOpen = false
			}
		})
		ui.Scroll(c).Grow(1).FillHeight().Children(func() {
			ui.Column(c).Padding(16).Gap(16).Children(func() {
				panel.draw(c, t)
			})
		})
	})
}

func panelHeaderButton(c *ui.Context, t *ui.Theme, label, icon string, danger bool) ui.Element {
	button := ui.Button(c, "").Size(26, 26).Padding(0).Radius(4).AlignItems(ui.Center).Justify(ui.Center).Tooltip(label).Label(label)
	button.Border(0, ui.Transparent)
	if button.Hovered() {
		button.Background(t.SurfacePressed)
	}
	if danger {
		button.TextColor(t.Danger)
	} else {
		button.TextColor(t.TextMuted)
	}
	button.Children(func() {
		ui.Icon(c, IconNamed(icon)).Size(14, 14)
	})
	return button
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

func (a *App) drawContextHeader(c *ui.Context, t *ui.Theme, current string) {
	if current == "" {
		ui.Text(c, "No context").FontSize(13).FontWeight(650).TextColor(t.TextMuted).SingleLine()
		return
	}
	ui.Text(c, current).FontSize(13).FontWeight(650).SingleLine().Tooltip(current)
}

func (a *App) drawSide(c *ui.Context, t *ui.Theme, current string) {
	page := ""
	if a.router != nil {
		page = screenID(a.router.Path())
	}
	ui.Column(c).Width(a.sideWidth()).Shrink(0).FillHeight().Background(t.Surface).Children(func() {
		ui.Row(c).Height(sideTitleH).Shrink(0).Padding(0, 12).AlignItems(ui.Center).Background(tabInactive(t)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Label("Sidebar header").Children(func() {
			a.drawContextHeader(c, t, current)
		})
		ui.Column(c).Shrink(0).Padding(8, 8, 4, 8).Children(func() {
			ui.TextInput(c, &a.menuQuery).Placeholder("Search").Grow(1)
		})
		ui.Scroll(c).Grow(1).FillHeight().Children(func() {
			ui.Column(c).Padding(4, 8, 8, 8).Gap(2).Children(func() {
				if len(a.contexts) > 0 {
					ui.Text(c, "CONTEXTS").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(8, 8, 2, 8)
					for _, name := range a.contexts {
						a.contextRow(c, t, name, name == current)
					}
				}
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
	if row.DoubleClicked() {
		a.showScreen(item.ID, false)
	} else if row.Clicked() {
		a.showScreen(item.ID, true)
	}
}

func (a *App) contextRow(c *ui.Context, t *ui.Theme, name string, active bool) {
	row := ui.Row(c).Height(sideRowH).Padding(0, 8).Gap(8).Radius(4).AlignItems(ui.Center)
	icon := t.TextMuted
	if active {
		row.Background(t.SurfacePressed)
		icon = t.Accent
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Icon(c, IconNamed("server")).Size(16, 16).TextColor(icon).Shrink(0)
		ui.Text(c, name).FontSize(13).SingleLine().MinWidth(0).Grow(1).Tooltip(name)
	})
	if !active && row.Clicked() {
		a.switchContext(name)
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
