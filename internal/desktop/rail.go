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
	a.mu.Lock()
	if a.catalogLoaded {
		a.mu.Unlock()
		return
	}
	a.catalogLoaded = true
	a.mu.Unlock()

	defaultPath, _ := cluster.DefaultKubeDir()
	state, err := readCatalog(a.session)
	if err != nil {
		a.mu.Lock()
		a.catalogDefault = defaultPath
		a.catalogErr = publicError(err)
		a.mu.Unlock()
		return
	}
	kinds := make(map[string]string, len(state.Files))
	for _, path := range state.Files {
		kinds[path] = catalogSourceKind(path)
	}
	a.mu.Lock()
	a.catalogDefault = defaultPath
	a.catalogFiles = state.Files
	a.catalogKinds = kinds
	a.mu.Unlock()

	for _, path := range state.Files {
		if err := a.addLoadedPath(path); err != nil {
			a.mu.Lock()
			if a.catalogErr == "" {
				a.catalogErr = publicError(err)
			}
			a.mu.Unlock()
		}
	}
}

func (a *App) addLoadedPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return a.session.SyncKubeDir(path)
	}
	return a.session.AddKubeconfigPath(path)
}

func (a *App) syncCatalog() {
	a.mu.Lock()
	files := append([]string(nil), a.catalogFiles...)
	a.mu.Unlock()
	if err := a.reloadCatalogSources(files); err != nil {
		a.mu.Lock()
		if a.catalogErr == "" {
			a.catalogErr = publicError(err)
		}
		a.mu.Unlock()
	}
	a.loadContexts()
	a.invalidate()
}

func (a *App) reloadCatalogSources(files []string) error {
	a.session.ClearKubeconfigPaths()
	var firstErr error
	for _, path := range files {
		if err := a.addLoadedPath(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if dir, err := cluster.DefaultKubeDir(); err == nil {
		if err := a.session.SyncKubeDir(dir); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (a *App) addCatalogPaths(paths []string) error {
	a.ensureCatalog()
	a.mu.Lock()
	original := append([]string(nil), a.catalogFiles...)
	a.mu.Unlock()
	next := append([]string(nil), original...)
	var added []string
	for _, path := range paths {
		path = filepath.Clean(path)
		if path == "." || containsPath(next, path) {
			continue
		}
		if err := a.addLoadedPath(path); err != nil {
			_ = a.reloadCatalogSources(original)
			return err
		}
		next = append(next, path)
		added = append(added, path)
	}
	if len(added) == 0 {
		return nil
	}
	if err := writeCatalog(a.session, catalogState{Files: next}); err != nil {
		_ = a.reloadCatalogSources(original)
		return err
	}
	a.mu.Lock()
	a.catalogFiles = next
	if a.catalogKinds == nil {
		a.catalogKinds = map[string]string{}
	}
	for _, path := range added {
		a.catalogKinds[path] = catalogSourceKind(path)
	}
	a.catalogErr = ""
	a.mu.Unlock()
	a.loadContexts()
	return nil
}

func (a *App) removeCatalogPath(path string) error {
	a.ensureCatalog()
	a.mu.Lock()
	next := make([]string, 0, len(a.catalogFiles))
	for _, file := range a.catalogFiles {
		if !sameCatalogPath(file, path) {
			next = append(next, file)
		}
	}
	if len(next) == len(a.catalogFiles) {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()
	if err := writeCatalog(a.session, catalogState{Files: next}); err != nil {
		return err
	}
	a.mu.Lock()
	a.catalogFiles = next
	delete(a.catalogKinds, path)
	a.catalogErr = ""
	a.mu.Unlock()
	err := a.reloadCatalogSources(next)
	a.loadContexts()
	return err
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if sameCatalogPath(path, want) {
			return true
		}
	}
	return false
}

func sameCatalogPath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && aa == bb
}

func catalogSourceKind(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "Unavailable"
	}
	if info.IsDir() {
		return "Folder"
	}
	return "File"
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

func writeCatalog(session *cluster.Session, state catalogState) error {
	path, err := catalogFile(session)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".catalog-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
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
