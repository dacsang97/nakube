package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/dacsang97/nakube/internal/cluster"
)

// catalogState is the persisted source list. Legacy files may still carry a
// "pins" key; encoding/json ignores unknown fields, so they parse cleanly.
type catalogState struct {
	Files []string `json:"files"`
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
