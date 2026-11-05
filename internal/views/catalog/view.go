package catalog

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Context is one kube context card.
type Context struct {
	Name   string
	Letter string
	Detail string
	Active bool
	Pinned bool
}

// Page is the catalog screen. Actions stay with the shell.
type Page struct {
	DirLabel string
	Err      string
	LoadErr  string
	Contexts []Context
	AddPath  *string
	Paste    *editor.State
	Pin      *ui.SVG
	OnDone   func()
	OnAdd    func()
	OnPaste  func()
	OnPin    func(name string)
	OnOpen   func(name string)
}

// Draw paints the catalog. It does not read the filesystem.
func Draw(c *ui.Context, t *ui.Theme, page Page) {
	ui.Scroll(c).Grow(1).FillHeight().Children(func() {
		ui.Column(c).Gap(16).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, "Catalog").FontSize(22).Bold()
					ui.Text(c, "Files in "+page.DirLabel+" show up as they appear. Added files stay outside that folder.").TextColor(t.TextMuted)
				})
				if ui.Button(c, "Done").Clicked() && page.OnDone != nil {
					page.OnDone()
				}
			})
			if page.Err != "" {
				ui.Text(c, page.Err).TextColor(t.Danger)
			}
			if page.LoadErr != "" {
				ui.Text(c, page.LoadErr).TextColor(t.Danger)
			}
			if len(page.Contexts) == 0 {
				ui.Text(c, "No contexts").TextColor(t.TextMuted)
			} else {
				ui.Row(c).Wrap().Gap(12).Children(func() {
					for _, info := range page.Contexts {
						card(c, t, page, info)
					}
				})
			}
			ui.Text(c, "Add a context").FontSize(16).Bold()
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Padding(12).Gap(8).Radius(12).Border(1, t.Border).Background(t.Surface).Children(func() {
					ui.Text(c, "From a file").Bold()
					ui.Text(c, "A kubeconfig file or a folder of them.").TextColor(t.TextMuted)
					ui.Row(c).Gap(8).Children(func() {
						ui.TextInput(c, page.AddPath).Placeholder("/path/to/kubeconfig or folder").Grow(1).MinWidth(0)
						if ui.Button(c, "Add file").Clicked() && page.OnAdd != nil {
							page.OnAdd()
						}
					})
				})
				ui.Column(c).Grow(1).MinWidth(0).Padding(12).Gap(8).Radius(12).Border(1, t.Border).Background(t.Surface).Children(func() {
					ui.Text(c, "Paste kubeconfig").Bold()
					if page.Paste != nil {
						editor.View(c, page.Paste).Height(96)
					}
					if ui.Button(c, "Add pasted kubeconfig").Clicked() && page.OnPaste != nil {
						page.OnPaste()
					}
				})
			})
		})
	})
}

func card(c *ui.Context, t *ui.Theme, page Page, info Context) {
	border := t.Border
	if info.Active {
		border = t.Accent
	}
	box := ui.Column(c).Width(280).Padding(14).Gap(8).Radius(12).Border(1, border).Background(t.Surface)
	box.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			avatar := ui.Box(c).Width(36).Height(36).Radius(8).Shrink(0).AlignItems(ui.Center).Justify(ui.Center)
			if info.Active {
				avatar.Background(t.Accent)
			} else {
				avatar.Background(t.SurfacePressed)
			}
			avatar.Children(func() {
				letter := ui.Text(c, info.Letter).Bold()
				if info.Active {
					letter.TextColor(t.AccentText)
				}
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, info.Name).Bold()
				if info.Active {
					ui.Badge(c, "Active")
				}
			})
			pin := "Pin"
			if info.Pinned {
				pin = "Unpin"
			}
			pinBtn := ui.Button(c, "").Size(28, 28).Padding(0).AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Tooltip(pin)
			pinBtn.Children(func() {
				if page.Pin != nil {
					ui.Icon(c, page.Pin).Size(14, 14)
				}
			})
			if pinBtn.Clicked() && page.OnPin != nil {
				page.OnPin(info.Name)
			}
		})
		detail := info.Detail
		if detail == "" {
			detail = "No server"
		}
		ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted)
	})
	if box.Clicked() && page.OnOpen != nil {
		page.OnOpen(info.Name)
	}
}
