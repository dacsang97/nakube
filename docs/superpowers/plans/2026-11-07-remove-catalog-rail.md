# Plan — Remove Catalog page, rail "+", pins (2026-11-07)

Branch: feat/context-dropdown · BASE: 5b59d7c · Decision record: docs/superpowers/plans/reports/brainstorm/2026-11-07-remove-catalog-rail.md

## Goal

The sidebar CONTEXTS list is the single place to switch contexts. The rail "+" button, the Catalog page, and pins are redundant UI — delete them. The cluster-side pipeline (persisted "files", ~/.kube sync, watcher) stays: contexts from kubeconfig files must keep appearing in the sidebar.

## Global Constraints

1. **Keep-list (do NOT remove):** `ensureCatalog`, `applyCatalogFiles`, `addLoadedPath`, `syncCatalog`, `startCatalogWatch`, `readCatalog`, `catalogFile`, `catalogDir`, `sessionState`, `catalogState` (Files only), fields `catalogLoaded`/`catalogFiles`/`catalogErr`; `togglePage` (Settings uses it); `activityButton` (Settings uses it); `IconNamed("plus")` in icons.go (tabbar.go:468 still uses it); the whole `internal/cluster` package (incl. `ImportKubeconfig`, `ContextInfos`, `AddKubeconfigPath`, `SyncKubeDir`, `RefreshLoadingRules`) — untouched.
2. `ensureCatalog` must keep the `a.ensureCatalog()` call in `drawActivityBar` (shell.go:137) and in Bootstrap (app.go:237) — never remove.
3. Old `catalog.json` files may contain `"pins"` — must keep parsing (json ignores unknown fields once `catalogState.Pins` is deleted).
4. Render-path state rules unchanged: no new unlocked reads beyond the accepted P2e class.
5. `go build ./...`, `go vet ./internal/...`, `gofmt -l internal/` clean; `go test ./internal/desktop` FULLY GREEN after this change (TestCatalogPage — the pinned env failure — is deleted with the page).
6. Conventional commit, exact message below. Single task, single commit.

## Task 1 — remove Catalog page, rail "+", pins; keep sync pipeline

### Step 1 — tests first (TDD)

**1a. Delete `TestCatalogPage`** (internal/desktop/app_test.go:513-523 — entire function).

**1b. In `TestSidebarIcons`** (app_test.go:680-699) delete the tail block:
```go
	if IconNamed("plus") == nil {
		t.Fatal("the rail has no plus icon")
	}
```

**1c. Add two tests after `TestContextRowClickStaysOnPage`** (ends app_test.go:~657):

```go
func TestCatalogRouteRemoved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("KUBECONFIG", filepath.Join(dir, "missing", "config")) // hermetic: no real kubeconfig
	s := cluster.NewSession()
	s.SetStatePath(filepath.Join(dir, "state", "context"))
	a := New(s)
	a.showScreen("catalog", true)
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("Not found") {
		t.Fatalf("catalog route still renders, texts %q", tt.Texts())
	}
}

func TestEnsureCatalogKeepsPersistedFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	kubeconfig := filepath.Join(dir, "extra.yaml")
	fixture := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: c
contexts:
- context:
    cluster: c
    user: u
  name: side-ctx
current-context: side-ctx
users:
- name: u
  user:
    token: test-token-not-for-logs
`
	if err := os.WriteFile(kubeconfig, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	statePath, err := cluster.StatePath()
	if err != nil {
		t.Fatal(err)
	}
	catFile := filepath.Join(filepath.Dir(statePath), "catalog.json")
	if err := os.MkdirAll(filepath.Dir(catFile), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"pins":["gone"],"files":["` + kubeconfig + `"]}`
	if err := os.WriteFile(catFile, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s := cluster.NewSession()
	a := New(s)
	a.ensureCatalog()
	a.syncCatalog()
	a.mu.Lock()
	have := append([]string(nil), a.contexts...)
	a.mu.Unlock()
	for _, want := range []string{"side-ctx"} {
		found := false
		for _, name := range have {
			if name == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("persisted file context %q missing, have %q", want, have)
		}
	}
}
```

Notes: `cluster.StatePath()` (exported, cluster/session.go:65-71) resolves under `os.UserConfigDir()` which honors `t.Setenv("HOME")`; `catalogFile` reads `filepath.Dir(statePath)/catalog.json` (rail.go:273-278 — `sessionState` ignores the session, pre-existing). The test writes exactly where the code reads. `a.syncCatalog()` is needed because `ensureCatalog` only registers files (`applyCatalogFiles` → `addLoadedPath`); contexts come from `loadContexts` inside `syncCatalog`.

### Step 2 — RED

```
go test ./internal/desktop -run 'TestCatalogRouteRemoved|TestEnsureCatalogKeepsPersistedFiles' -v
```
Expected: `TestCatalogRouteRemoved` **FAILS** — catalog route still serves the Catalog page (texts contain "Catalog"/"Done", no "Not found"). `TestEnsureCatalogKeepsPersistedFiles` **PASSES** already (it is a regression guard, not a RED test).

If RED differs, stop and report BLOCKED.

### Step 3 — implementation

**3a. Rewrite `internal/desktop/rail.go`** to exactly this (removes: `drawCatalog`, `kubeDirLabel`, `ensurePaste`, `contextDetail`, `addCatalogPath`, `importPaste`, `togglePin`, `pinned`, `syncCatalogIfDue`, `saveCatalog`, `catalogState.Pins`, and the now-unused `catalog`/`editor`/`strings`/`time`/`utf8` imports; keeps the sync pipeline byte-identical):

```go
package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

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
```

**3b. `internal/desktop/app.go`** — remove these lines:
- `:154` `	catalogSynced     time.Time`
- `:155` `	catalogPins       []string`
- `:158` `	addPath           string`
- `:159` `	paste             *editor.State`
- `:501-503` route case:
```go
	case r.Match("/catalog"):
		r.Title("Catalog")
		a.drawCatalog(c, t, current)
```
- `:593-597` entire `func (a *App) closeCatalog()`.

**3c. `internal/desktop/shell.go`** — replace `drawActivityBar` (shell.go:136-160) and delete `pinButton` (its whole func, currently ~186-204, body contains `a.switchContext(name); a.openPage("overview")`):

```go
func (a *App) drawActivityBar(c *ui.Context, t *ui.Theme) {
	a.ensureCatalog()
	page := ""
	if a.router != nil {
		page = screenID(a.router.Path())
	}
	ui.Column(c).Width(activityBarW).FillHeight().Shrink(0).Background(t.Surface).BorderEndWidth(1).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		ui.Spacer(c).Grow(1)
		ui.Column(c).Shrink(0).Padding(4, 0, 8, 0).AlignItems(ui.Center).Children(func() {
			a.activityButton(c, t, "settings", "Settings", page == "settings", func() {
				a.togglePage("/settings")
			})
		})
	})
}
```
(`ui.Spacer` exists — mygo api_gen.go:1994. The old Scroll+pins column collapses away. Signature drops `current`.)

**3d. `internal/desktop/app.go:467`** — update the call: `a.drawActivityBar(c, t, current)` → `a.drawActivityBar(c, t)`.

**3e. Delete the package:** `git rm -r internal/views/catalog` (only `view.go`, 136 lines; only rail.go imported it — that import is gone in 3a).

### Step 4 — GREEN + package + static checks

```
go test ./internal/desktop -run 'TestCatalogRouteRemoved|TestEnsureCatalogKeepsPersistedFiles' -v   # both PASS
go build ./... && go vet ./internal/... && gofmt -l internal/                                       # all clean
go test ./internal/...                                                                              # FULLY GREEN (no pinned failures left in desktop)
```
Also verify the keep-list is still wired: `grep -n "ensureCatalog\|syncCatalog\|startCatalogWatch" internal/desktop/app.go` shows Bootstrap calls (app.go:237-241).

### Step 5 — commit

```
git add internal/desktop/rail.go internal/desktop/app.go internal/desktop/shell.go internal/desktop/app_test.go
git rm -r internal/views/catalog   # if not already staged in 3e
git commit -m "refactor(desktop): remove catalog page and rail plus button"
```

## Review Focus

- Keep-list intact: Bootstrap pipeline (ensureCatalog → syncCatalog → startCatalogWatch) untouched; `catalogState` loses only Pins; no helper in the keep-list lost its callers.
- No dangling references: `grep -rn "drawCatalog\|pinButton\|catalogPins\|closeCatalog\|togglePin\|addCatalogPath\|importPaste\|ensurePaste\|syncCatalogIfDue\|railLabel\|contextDetail\|kubeDirLabel\|containsString\|views/catalog" internal/` → only the new tests' absence, zero hits in non-test code.
- Old catalog.json compat: `{"pins":[...],"files":[...]}` parses (json ignores unknown keys) — pinned by TestEnsureCatalogKeepsPersistedFiles.
- TestCatalogPage gone → desktop package fully green.
- drawActivityBar: Settings button still highlighted on /settings; Spacer keeps it bottom-anchored; no `current` param left.
