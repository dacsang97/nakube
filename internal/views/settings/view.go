package settings

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// CatalogItem is the public, credential-free part of one kubeconfig context.
type CatalogItem struct {
	Name      string
	Cluster   string
	Namespace string
	Server    string
}

// CatalogSource is one file or folder watched for kubeconfig changes.
type CatalogSource struct {
	Path      string
	Kind      string
	Removable bool
}

type CatalogActionKind uint8

const (
	CatalogNone CatalogActionKind = iota
	CatalogSyncFiles
	CatalogSyncFolder
	CatalogPaste
	CatalogRemove
)

type CatalogAction struct {
	Kind CatalogActionKind
	Path string
}

// Index is the segmented position of a theme source.
func Index(source mygo.ThemeSource) int {
	switch source {
	case mygo.ThemeLight:
		return 1
	case mygo.ThemeDark:
		return 2
	default:
		return 0
	}
}

// FromIndex maps a segmented position back to a theme source.
func FromIndex(index int) mygo.ThemeSource {
	switch index {
	case 1:
		return mygo.ThemeLight
	case 2:
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}

// DrawCatalog manages kubeconfig sync sources and shows their contexts.
func DrawCatalog(c *ui.Context, t *ui.Theme, sources []CatalogSource, items []CatalogItem, current, errText string, busy bool) CatalogAction {
	action := CatalogAction{}
	page(c, t, "Catalog", "Manage the kubeconfig files and folders NaKube loads.", func() {
		if errText != "" {
			ui.Text(c, errText).TextColor(t.Danger).Wrap()
		}
		card(c, t, func() {
			ui.Text(c, "Manage Kubeconfigs").FontSize(16).Bold()
			ui.Text(c, "Sync individual kubeconfig files, every kubeconfig in a folder, or paste a config directly.").TextColor(t.TextMuted).Wrap()
			ui.Row(c).Gap(8).Children(func() {
				files := ui.Button(c, "Sync files").Disabled(busy)
				if files.Clicked() {
					action.Kind = CatalogSyncFiles
				}
				folder := ui.Button(c, "Sync folder").Disabled(busy)
				if folder.Clicked() {
					action.Kind = CatalogSyncFolder
				}
				paste := ui.Button(c, "Paste kubeconfig").Disabled(busy)
				if paste.Clicked() {
					action.Kind = CatalogPaste
				}
			})
		})
		card(c, t, func() {
			ui.Text(c, "Synced Items").FontSize(16).Bold()
			if len(sources) == 0 {
				ui.Text(c, "No files and folders have been synced yet.").TextColor(t.TextMuted)
				return
			}
			for _, source := range sources {
				source := source
				ui.Row(c).MinHeight(38).Padding(6, 8).Gap(10).Radius(8).Background(t.SurfaceHover).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, source.Path).FontSize(13).SingleLine().Tooltip(source.Path)
						ui.Text(c, source.Kind).FontSize(11).TextColor(t.TextMuted)
					})
					if source.Removable {
						remove := ui.Button(c, "Remove").Padding(4, 8).Disabled(busy).Label("Remove " + source.Path)
						if remove.Clicked() {
							action = CatalogAction{Kind: CatalogRemove, Path: source.Path}
						}
					} else {
						ui.Text(c, "Built-in").FontSize(11).TextColor(t.TextMuted)
					}
				})
			}
		})
		card(c, t, func() {
			ui.Text(c, "Contexts").FontSize(16).Bold()
			if len(items) == 0 {
				ui.Text(c, "No contexts found in the synced kubeconfigs.").TextColor(t.TextMuted)
				return
			}
			for _, item := range items {
				item := item
				ui.Column(c).Padding(8).Gap(3).Radius(8).Background(t.SurfaceHover).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, item.Name).FontSize(14).Bold().SingleLine().Tooltip(item.Name)
						if item.Name == current {
							ui.Text(c, "Current").FontSize(11).TextColor(t.Success)
						}
					})
					detail := item.Cluster
					if item.Namespace != "" {
						detail += " · " + item.Namespace
					}
					if detail != "" {
						ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted).SingleLine().Tooltip(detail)
					}
					if item.Server != "" {
						ui.Text(c, item.Server).FontSize(11).TextColor(t.TextMuted).SingleLine().Tooltip(item.Server)
					}
				})
			}
		})
	})
	return action
}

// DrawTheme shows the existing appearance controls.
func DrawTheme(c *ui.Context, t *ui.Theme, index *int, current mygo.ThemeSource) {
	page(c, t, "Theme", "Choose how NaKube looks on this computer.", func() {
		card(c, t, func() {
			ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, "Appearance").FontSize(16).Bold()
					ui.Text(c, "System follows this computer. Light and Dark apply to the app.").TextColor(t.TextMuted).Wrap()
				})
				ui.Segmented(c, index, "System", "Light", "Dark").Shrink(0).Label("Appearance")
			})
			ui.Row(c).Gap(12).Children(func() {
				preview(c, t, "Light", ui.LightTheme(), current == mygo.ThemeLight)
				preview(c, t, "Dark", ui.DarkTheme(), current == mygo.ThemeDark)
			})
		})
	})
}

// DrawMobile shows the Tailscale endpoint that NaKube Mobile can scan.
func DrawMobile(c *ui.Context, t *ui.Theme, qr *ui.Bitmap, address, errText string) {
	page(c, t, "Na Kube Mobile", "Connect a phone on the same Tailscale network.", func() {
		card(c, t, func() {
			ui.Row(c).AlignItems(ui.Start).Gap(24).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(10).Children(func() {
					ui.Text(c, "Pair this computer").FontSize(16).Bold()
					ui.Text(c, "Open Na Kube Mobile, tap Scan QR Code, and scan the code shown here. Both devices must be signed in to the same tailnet.").TextColor(t.TextMuted).Wrap()
					if errText != "" {
						ui.Text(c, errText).TextColor(t.Danger).Wrap()
						return
					}
					if address == "" {
						ui.Text(c, "Checking Tailscale…").TextColor(t.TextMuted)
						return
					}
					ui.Text(c, address).FontSize(13).SingleLine().Tooltip(address)
					if ui.Button(c, "Copy address").Clicked() {
						c.WriteClipboard(address)
					}
				})
				ui.Column(c).Width(196).Shrink(0).AlignItems(ui.Center).Gap(8).Children(func() {
					if qr != nil {
						ui.Box(c).Size(196, 196).Padding(10).Radius(12).Background(ui.RGB(255, 255, 255)).Children(func() {
							ui.Image(c, qr).Size(176, 176)
						})
						ui.Text(c, "Scan with Na Kube Mobile").FontSize(12).TextColor(t.TextMuted)
					}
				})
			})
		})
	})
}

func page(c *ui.Context, t *ui.Theme, title, subtitle string, body func()) {
	ui.Scroll(c).Grow(1).FillHeight().Children(func() {
		ui.Column(c).MaxWidth(720).Gap(16).Children(func() {
			ui.Text(c, title).FontSize(22).Bold()
			ui.Text(c, subtitle).TextColor(t.TextMuted).Wrap()
			body()
		})
	})
}

func card(c *ui.Context, t *ui.Theme, body func()) {
	ui.Column(c).Padding(16).Gap(14).Radius(12).Background(t.Surface).Border(1, t.Border).Children(body)
}

func preview(c *ui.Context, t *ui.Theme, name string, sample *ui.Theme, chosen bool) {
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
