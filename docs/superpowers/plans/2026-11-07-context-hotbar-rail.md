# Plan: move the context list into the activity rail (Lens Hotbar)

Branch: feat/context-dropdown · Base: c8e1056 · Model tier for implementer/reviewer: zai/glm-5.3-flash (plan carries full code)

## Global Constraints
- NEVER touch the `switchContext` guard (`internal/desktop/app.go:~416-418`, text: `if name == "" || name == a.session.Current() { return }`) or anything in `internal/cluster`.
- NEVER touch the Bootstrap pipeline (`ensureCatalog → syncCatalog → startCatalogWatch`, app.go:233-241) — load-bearing for context loading.
- `contextRow`'s click contract carries over to the rail: switch-only, NO `openPage`/`showScreen` on click.
- drawSide/drawActivityBar read ONLY the snapshot params (`current`, `contexts`) — same critical section in `view()` (app.go:~455-456). No new `a.` field reads in draw code.
- Tests that build apps and touch context switching keep the hermetic pattern: `t.Setenv("HOME", dir)` + `t.Setenv("KUBECONFIG", ...)` pointing at the fixture written into `t.TempDir()` + `s.SetStatePath(filepath.Join(dir, "state", "context"))` (KUBECONFIG set AFTER os.WriteFile — round-4 lesson).
- Pinned env failure `TestContextInfoHidesCredentials` (internal/cluster) is out of scope; `internal/desktop` must end FULLY GREEN.
- `.Label(name)` chain is already used at shell.go:110 (`ui.Button(c, "")...Tooltip(label).Label(label)`) — no new mygo API risk.
- Commit message (exact): `feat(desktop): move context list into the activity rail`
- Files allowed to change: internal/desktop/shell.go, internal/desktop/app.go, internal/desktop/app_test.go, plus new docs commits by the controller. NOTHING else (do not push).

## Task 1 (only task)

### Step 1 — tests first (RED)
In `internal/desktop/app_test.go`:

1a. DELETE `TestContextsFilterBySearch` (lines 473-491) — feature retires with the sidebar list.

1b. REWRITE `TestMenuSearchFilters` (493-511): drop the `a.contexts`/`a.current` seeds and the two trailing blocks (the `dev` and `CONTEXTS` assertions). Final body:
```go
func TestMenuSearchFilters(t *testing.T) {
	a := New(cluster.NewSession())
	a.menuQuery = "virtual"
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("VirtualServices") {
		t.Fatalf("missing match, texts %q", tt.Texts())
	}
	if tt.HasText("DaemonSets") || tt.HasText("Workloads") {
		t.Fatalf("unrelated item stayed, texts %q", tt.Texts())
	}
}
```

1c. REWRITE `TestContextRowClickStaysOnPage` (starts line 587) into `TestRailContextClickStaysOnPage` — keep the fixture + hermetic env EXACTLY as-is (HOME + KUBECONFIG-after-WriteFile + SetStatePath), change only the middle: keep `a.contexts = []string{"dev", "prod"}`, `a.current = "dev"`, `a.router.Push("/pods")`; `tt.Click("prod")` now targets the rail button's LABEL; keep both assertions (switched to prod AND router stays "/pods"). Rename only the function.

1d. ADD `TestRailListsContexts` after `TestSideHeaderNoList`:
```go
func TestRailListsContexts(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = []string{"dev", "prod"}
	a.current = "dev"
	tt := ui.NewTester(a.view, 1100, 720)
	for _, name := range []string{"dev", "prod"} {
		if _, ok := tt.Find(name); !ok {
			t.Fatalf("rail misses context %q (label), texts %q", name, tt.Texts())
		}
	}
	if tt.HasText("CONTEXTS") {
		t.Fatalf("sidebar contexts section survived the move, texts %q", tt.Texts())
	}
}
```
(No kubeconfig needed: rail renders from the snapshots; `New(cluster.NewSession())` is the file's standard cheap app.)

RED expectation: `TestRailContextClickStaysOnPage` FAILS (`ui: no element shows "prod"` — rail buttons don't exist yet) and `TestRailListsContexts` FAILS (`rail misses context "dev"`); everything else unchanged-green. Anything different → BLOCKED with exact output.

### Step 2 — implement (GREEN)

2a. `internal/desktop/shell.go` — REPLACE `drawActivityBar` (lines 136-151) with:
```go
func (a *App) drawActivityBar(c *ui.Context, t *ui.Theme, current string, contexts []string) {
	a.ensureCatalog()
	page := ""
	if a.router != nil {
		page = screenID(a.router.Path())
	}
	ui.Column(c).Width(activityBarW).FillHeight().Shrink(0).Background(t.Surface).BorderEndWidth(1).BorderColor(t.Border).AlignItems(ui.Center).Children(func() {
		ui.Scroll(c).Grow(1).FillHeight().Shrink(0).Children(func() {
			ui.Column(c).Padding(4, 0).Gap(4).AlignItems(ui.Center).Children(func() {
				for _, name := range contexts {
					a.railContextButton(c, t, name, name == current)
				}
			})
		})
		ui.Column(c).Shrink(0).Padding(4, 0, 8, 0).AlignItems(ui.Center).Children(func() {
			a.activityButton(c, t, "settings", "Settings", page == "settings", func() {
				a.togglePage("/settings")
			})
		})
	})
}
```

2b. `internal/desktop/shell.go` — ADD `railContextButton` immediately after `activityButton`'s closing brace (before `drawSide`):
```go
func (a *App) railContextButton(c *ui.Context, t *ui.Theme, name string, active bool) {
	slot := ui.Box(c).Height(activitySlot).AlignSelf(ui.Stretch).AlignItems(ui.Center).Justify(ui.Center)
	slot.Children(func() {
		btn := ui.Button(c, "").Size(32, 32).Padding(0).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Tooltip(name).Label(name)
		if active {
			btn.Background(t.SurfacePressed)
		} else if btn.Hovered() {
			btn.Background(t.SurfaceHover)
		}
		btn.Children(func() {
			ui.Icon(c, IconNamed("server")).Size(20, 20).TextColor(contextColor(name))
		})
		if !active && btn.Clicked() {
			a.switchContext(name)
		}
		if active {
			ui.Box(c).Absolute().Left(22).Top(22).Width(6).Height(6).Radius(3).Background(t.Success)
		}
	})
}
```
(The active identity color is NOT overwritten by accent — identity color IS the identifier in the rail; this also retires carried minor P3c's half-fix opportunity by design. Dot is a sibling of the button inside the slot, absolute-positioned like activityButton's active bar at shell.go:155-157.)

2c. `internal/desktop/shell.go` — DELETE the whole `contextRow` func (lines 273-297).

2d. `internal/desktop/shell.go` — in `drawSide` (line 186): change signature to `func (a *App) drawSide(c *ui.Context, t *ui.Theme, current string)` and DELETE the CONTEXTS block inside the scroll column (from `if len(contexts) > 0 {` through its closing brace, ending right before `shown := 0`). `strings` import stays (menu loop uses `strings.ToUpper`).

2e. `internal/desktop/app.go` — call site (~line 466-467) becomes:
```go
			a.drawActivityBar(c, t, current, contexts)
			a.drawSide(c, t, current)
```
(`current` and `contexts` both come from the existing view() snapshot — no other change in app.go.)

### Step 3 — gates (all must pass, in order)
- `go test ./internal/desktop -run 'TestRailContextClickStaysOnPage|TestRailListsContexts|TestMenuSearchFilters|TestSideHeaderSelectsContext|TestContextRowClickStaysOnPage' -v` — note `TestContextRowClickStaysOnPage` must now FAIL-TO-COMPILE-FREE: it no longer exists (renamed); expected: 4 tests pass.
- `go build ./...` clean; `go vet ./internal/...` clean; `gofmt -l internal/` empty.
- `go test ./internal/desktop` FULLY GREEN (zero failures).
- `go test ./internal/cluster` → only pinned `TestContextInfoHidesCredentials`.
- Sanity greps, zero hits in non-test code: `contextRow\b`, `"CONTEXTS"` (the header literal), `drawSide(c, t, current, contexts)`.
- `git show --stat` = exactly 3 files: shell.go, app.go, app_test.go.

### Step 4 — commit
```
git add internal/desktop/shell.go internal/desktop/app.go internal/desktop/app_test.go
git commit -m "feat(desktop): move context list into the activity rail"
```
No push. Report status, RED/GREEN outputs, commit hash, stat, concerns.
