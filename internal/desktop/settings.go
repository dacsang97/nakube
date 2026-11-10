package desktop

import (
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/settings"
)

type settingsNavItem struct {
	ID    string
	Label string
	Icon  string
}

var settingsNav = []settingsNavItem{
	{ID: "catalog", Label: "Catalog", Icon: "database"},
	{ID: "theme", Label: "Theme", Icon: "settings"},
	{ID: "metrics", Label: "Metrics", Icon: "gauge"},
	{ID: "mobile", Label: "Na Kube Mobile", Icon: "server"},
}

func (a *App) selectedSettingsPage() string {
	for _, item := range settingsNav {
		if a.settingsPage == item.ID {
			return item.ID
		}
	}
	return "catalog"
}

func (a *App) selectedSettingsTitle() string {
	selected := a.selectedSettingsPage()
	for _, item := range settingsNav {
		if item.ID == selected {
			return item.Label
		}
	}
	return "Catalog"
}

func (a *App) drawSettingsSide(c *ui.Context, t *ui.Theme) {
	selected := a.selectedSettingsPage()
	ui.Column(c).Width(a.sideWidth()).Shrink(0).FillHeight().Background(t.Surface).Children(func() {
		ui.Row(c).Height(sideTitleH).Shrink(0).Padding(0, 12).AlignItems(ui.Center).Background(tabInactive(t)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Label("Sidebar header").Children(func() {
			ui.Text(c, "Settings").FontSize(13).FontWeight(650).SingleLine()
		})
		ui.Scroll(c).Grow(1).FillHeight().Children(func() {
			ui.Column(c).Padding(8).Gap(2).Children(func() {
				for _, item := range settingsNav {
					item := item
					row := ui.Row(c).Height(sideRowH).Padding(0, 8).Gap(8).Radius(4).AlignItems(ui.Center)
					iconColor := t.TextMuted
					if item.ID == selected {
						row.Background(t.SurfacePressed)
						iconColor = t.Accent
					} else if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					row.Children(func() {
						ui.Icon(c, IconNamed(item.Icon)).Size(16, 16).TextColor(iconColor).Shrink(0)
						ui.Text(c, item.Label).FontSize(13).SingleLine().MinWidth(0).Grow(1)
					})
					if row.Clicked() && item.ID != selected {
						a.settingsPage = item.ID
						a.invalidate()
					}
				}
			})
		})
		a.sideResize(c)
	})
}

// drawSettings is a page of this window. It is not a second window.
func (a *App) drawSettings(c *ui.Context, t *ui.Theme) {
	switch a.selectedSettingsPage() {
	case "theme":
		index := settings.Index(a.appearance)
		settings.DrawTheme(c, t, &index, a.appearance)
		if next := settings.FromIndex(index); next != a.appearance {
			a.applyAppearance(next, true)
		}
	case "metrics":
		a.drawMetricsSettings(c, t)
	case "mobile":
		a.mu.Lock()
		address, errText, qr := a.mobileAddress, a.mobileErr, a.mobileQR
		a.mu.Unlock()
		settings.DrawMobile(c, t, qr, address, errText)
	default:
		a.ensureCatalog()
		a.mu.Lock()
		infos := append([]cluster.ContextInfo(nil), a.contextInfos...)
		files := append([]string(nil), a.catalogFiles...)
		kinds := make(map[string]string, len(a.catalogKinds))
		for path, kind := range a.catalogKinds {
			kinds[path] = kind
		}
		defaultPath := a.catalogDefault
		current, errText, busy := a.current, a.catalogErr, a.catalogBusy
		a.mu.Unlock()
		sources := make([]settings.CatalogSource, 0, len(files)+1)
		if defaultPath != "" {
			sources = append(sources, settings.CatalogSource{Path: defaultPath, Kind: "Built-in folder"})
		}
		for _, path := range files {
			sources = append(sources, settings.CatalogSource{Path: path, Kind: kinds[path], Removable: true})
		}
		items := make([]settings.CatalogItem, len(infos))
		for i := range infos {
			items[i] = settings.CatalogItem{
				Name:      infos[i].Name,
				Cluster:   infos[i].Cluster,
				Namespace: infos[i].Namespace,
				Server:    infos[i].Server,
			}
		}
		a.handleCatalogAction(settings.DrawCatalog(c, t, sources, items, current, errText, busy))
		a.drawCatalogPasteDialog(c, t)
	}
}

func (a *App) handleCatalogAction(action settings.CatalogAction) {
	switch action.Kind {
	case settings.CatalogSyncFiles:
		a.pickCatalogPaths(false)
	case settings.CatalogSyncFolder:
		a.pickCatalogPaths(true)
	case settings.CatalogPaste:
		a.catalogPasteOpen = true
		a.invalidate()
	case settings.CatalogRemove:
		a.runCatalogChange(func() error { return a.removeCatalogPath(action.Path) })
	}
}

func (a *App) drawCatalogPasteDialog(c *ui.Context, t *ui.Theme) {
	if !a.catalogPasteOpen {
		return
	}
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(640).MaxWidthPercent(90).Padding(18).Gap(12).Children(func() {
			ui.Text(c, "Paste kubeconfig").FontSize(18).Bold()
			ui.Text(c, "Paste a kubeconfig document. NaKube validates it before saving it to the private catalog directory.").TextColor(t.TextMuted).Wrap()
			ui.TextArea(c, &a.catalogPaste).Placeholder("apiVersion: v1\nkind: Config\n…").Label("Kubeconfig YAML").Lines(14, 24).FillWidth()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "Cancel").Clicked() {
					a.catalogPaste = ""
					open = false
				}
				add := ui.Button(c, "Add kubeconfig").Disabled(strings.TrimSpace(a.catalogPaste) == "")
				if add.Clicked() {
					body := a.catalogPaste
					a.catalogPaste = ""
					open = false
					a.runCatalogChange(func() error { return a.importCatalog(body) })
				}
			})
		})
	})
	a.catalogPasteOpen = open
}

func (a *App) importCatalog(body string) error {
	dir, err := catalogDir(a.session)
	if err != nil {
		return err
	}
	path, err := a.session.ImportKubeconfig(dir, body)
	if err != nil {
		return err
	}
	if err := a.addCatalogPaths([]string{path}); err != nil {
		a.session.RemoveKubeconfigPath(path)
		_ = os.Remove(path)
		return err
	}
	return nil
}

func (a *App) pickCatalogPaths(directory bool) {
	if !a.beginCatalogChange() {
		return
	}
	title := "Sync kubeconfig files"
	if directory {
		title = "Sync kubeconfig folders"
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:          a.window,
			Title:           title,
			Directory:       directory,
			Multiple:        true,
			ShowHiddenFiles: true,
		})
		if err == nil && len(paths) > 0 {
			err = a.addCatalogPaths(paths)
		}
		a.finishCatalogChange(err)
	}()
}

func (a *App) runCatalogChange(change func() error) {
	if !a.beginCatalogChange() {
		return
	}
	go func() {
		a.finishCatalogChange(change())
	}()
}

func (a *App) beginCatalogChange() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.catalogBusy {
		return false
	}
	a.catalogBusy = true
	a.catalogErr = ""
	return true
}

func (a *App) finishCatalogChange(err error) {
	a.mu.Lock()
	a.catalogBusy = false
	if err != nil {
		a.catalogErr = publicError(err)
	}
	a.mu.Unlock()
	a.invalidate()
}

// drawMetricsSettings edits the open context's Prometheus service.
func (a *App) drawMetricsSettings(c *ui.Context, t *ui.Theme) {
	contextName := a.session.Current()
	a.mu.Lock()
	saved := a.promOverridesLocked()[contextName]
	a.mu.Unlock()
	if a.promDraftFor != contextName {
		a.promDraftFor, a.promDraft, a.promErr = contextName, saved, ""
	}
	shownErr := ""
	if a.promErrFor == a.promDraft {
		shownErr = a.promErr
	}
	prom := settings.Prometheus{Context: contextName, Draft: &a.promDraft, Saved: saved, Err: shownErr}
	if settings.DrawMetrics(c, t, prom) {
		a.promErr, a.promErrFor = a.savePromOverride(contextName, a.promDraft), a.promDraft
		if a.promErr == "" {
			a.mu.Lock()
			a.promDraft = a.promOverridesLocked()[contextName]
			a.mu.Unlock()
		}
	}
}
