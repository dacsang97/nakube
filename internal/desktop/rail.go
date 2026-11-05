package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/views/catalog"
)

type catalogState struct {
	Pins  []string `json:"pins"`
	Files []string `json:"files"`
}

func (a *App) drawCatalog(c *ui.Context, t *ui.Theme, current string) {
	a.ensureCatalog()
	a.syncCatalogIfDue()
	a.ensurePaste()
	dir, _ := cluster.DefaultKubeDir()
	page := catalog.Page{
		DirLabel: kubeDirLabel(dir),
		Err:      a.catalogErr,
		AddPath:  &a.addPath,
		Paste:    a.paste,
		Pin:      IconNamed("pin"),
		OnDone:   a.closeCatalog,
		OnAdd:    a.addCatalogPath,
		OnPaste:  a.importPaste,
		OnPin:    a.togglePin,
		OnOpen: func(name string) {
			a.switchContext(name)
			a.openPage("overview")
		},
	}
	infos, _, err := a.session.ContextInfos()
	if err != nil {
		page.LoadErr = publicError(err)
		infos = nil
	}
	for _, info := range infos {
		page.Contexts = append(page.Contexts, catalog.Context{
			Name:   info.Name,
			Letter: railLabel(info.Name),
			Detail: contextDetail(info),
			Active: info.Name == current,
			Pinned: a.pinned(info.Name),
		})
	}
	catalog.Draw(c, t, page)
}

func kubeDirLabel(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil || dir == "" {
		return "~/.kube"
	}
	rel, err := filepath.Rel(home, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return dir
	}
	return "~/" + filepath.ToSlash(rel)
}

func (a *App) ensurePaste() {
	if a.paste != nil {
		return
	}
	buf := editor.NewBuffer("")
	buf.SetLanguage(editor.LanguageFor("kubeconfig.yaml"))
	a.paste = &editor.State{Buffer: buf}
}

func contextDetail(info cluster.ContextInfo) string {
	var parts []string
	if info.Namespace != "" {
		parts = append(parts, info.Namespace)
	}
	switch {
	case info.Server != "":
		parts = append(parts, info.Server)
	case info.Cluster != "" && info.Cluster != info.Name:
		parts = append(parts, info.Cluster)
	}
	return strings.Join(parts, "  ·  ")
}

func (a *App) addCatalogPath() {
	path := strings.TrimSpace(a.addPath)
	if err := a.session.AddKubeconfigPath(path); err != nil {
		a.catalogErr = publicError(err)
		return
	}
	if !containsString(a.catalogFiles, path) {
		a.catalogFiles = append(a.catalogFiles, path)
	}
	a.catalogErr = ""
	a.addPath = ""
	_ = a.saveCatalog()
}

func (a *App) importPaste() {
	if a.paste == nil || a.paste.Buffer == nil {
		return
	}
	dir, err := catalogDir(a.session)
	if err != nil {
		a.catalogErr = publicError(err)
		return
	}
	path, err := a.session.ImportKubeconfig(dir, a.paste.Buffer.String())
	if err != nil {
		a.catalogErr = publicError(err)
		return
	}
	if !containsString(a.catalogFiles, path) {
		a.catalogFiles = append(a.catalogFiles, path)
	}
	a.catalogErr = ""
	_ = a.saveCatalog()
}

func (a *App) togglePin(name string) {
	if a.pinned(name) {
		next := a.catalogPins[:0]
		for _, pin := range a.catalogPins {
			if pin != name {
				next = append(next, pin)
			}
		}
		a.catalogPins = next
	} else {
		a.catalogPins = append(a.catalogPins, name)
	}
	_ = a.saveCatalog()
}

func (a *App) pinned(name string) bool {
	return containsString(a.catalogPins, name)
}

func (a *App) ensureCatalog() {
	if a.catalogLoaded {
		return
	}
	a.catalogLoaded = true
	state, err := readCatalog(a.session)
	if err != nil {
		a.catalogErr = publicError(err)
		return
	}
	a.catalogPins = state.Pins
	a.catalogFiles = state.Files
	a.applyCatalogFiles()
}

func (a *App) applyCatalogFiles() {
	for _, path := range a.catalogFiles {
		a.addLoadedPath(path)
	}
}

func (a *App) addLoadedPath(path string) {
	info, err := os.Stat(path)
	if err != nil {
		if a.catalogErr == "" {
			a.catalogErr = publicError(err)
		}
		return
	}
	var addErr error
	if info.IsDir() {
		addErr = a.session.SyncKubeDir(path)
	} else {
		addErr = a.session.AddKubeconfigPath(path)
	}
	if addErr != nil && a.catalogErr == "" {
		a.catalogErr = publicError(addErr)
	}
}

func (a *App) syncCatalogIfDue() {
	a.mu.Lock()
	due := a.catalogSynced.IsZero() || time.Since(a.catalogSynced) >= time.Second
	if due {
		a.catalogSynced = time.Now()
	}
	a.mu.Unlock()
	if !due {
		return
	}
	a.syncCatalog()
}

func (a *App) syncCatalog() {
	a.session.RefreshLoadingRules()
	a.mu.Lock()
	files := append([]string(nil), a.catalogFiles...)
	a.mu.Unlock()
	for _, path := range files {
		a.addLoadedPath(path)
	}
	if dir, err := cluster.DefaultKubeDir(); err == nil {
		_ = a.session.SyncKubeDir(dir)
	}
	a.loadContexts()
	a.invalidate()
}

func (a *App) startCatalogWatch(ctx context.Context) {
	dir, err := cluster.DefaultKubeDir()
	if err != nil {
		return
	}
	w, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{})
	if err != nil {
		return
	}
	go func() {
		defer w.Close()
		for {
			if _, err := w.Next(ctx); err != nil {
				return
			}
			a.syncCatalog()
		}
	}()
}

func (a *App) saveCatalog() error {
	path, err := catalogFile(a.session)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.Marshal(catalogState{Pins: a.catalogPins, Files: a.catalogFiles})
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

func readCatalog(session *cluster.Session) (catalogState, error) {
	path, err := catalogFile(session)
	if err != nil {
		return catalogState{}, err
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return catalogState{}, nil
	}
	if err != nil {
		return catalogState{}, err
	}
	var state catalogState
	if err := json.Unmarshal(body, &state); err != nil {
		return catalogState{}, err
	}
	return state, nil
}

func catalogFile(session *cluster.Session) (string, error) {
	dir, err := catalogDir(session)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(dir), "catalog.json"), nil
}

func catalogDir(session *cluster.Session) (string, error) {
	path, err := sessionState(session)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "kubeconfigs"), nil
}

func sessionState(session *cluster.Session) (string, error) {
	if session == nil {
		return cluster.StatePath()
	}
	return cluster.StatePath()
}

func railLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	r, size := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError && size == 1 {
		return "?"
	}
	return strings.ToUpper(string(r))
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
