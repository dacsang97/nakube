# Context list in sidebar — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the sidebar-header context dropdown with a clickable list of contexts at the top of the left sidebar.

**Architecture:** The header (`drawContextHeader` in `internal/desktop/shell.go`) drops its `ui.Select` and becomes a static label. The sidebar's scroll column gains a "CONTEXTS" section above the menu sections, rendered by a new `contextRow` that mirrors `sideRow` styling. Clicking a non-active row calls the existing `switchContext(name)` + `openPage("overview")` — identical to today's dropdown and rail-pin behavior.

**Tech Stack:** Go 1.27, mygo (`github.com/egoist/mygo`) UI toolkit, client-go.

**Spec:** `docs/superpowers/specs/2026-11-06-context-list-sidebar-design.md`

## Global Constraints

- Draw functions read App fields directly without locking (established pattern in `drawSide`); no mutex acquisition inside draw.
- Contexts come from cached `a.contexts []string`; active context is `a.current string`. No session queries inside draw.
- Context rows are always visible — the menu search (`a.menuQuery` / `menuItems`) must not filter them.
- Long context names truncate to one line; full name via tooltip on the row label and on the header text.
- `switchContext`'s early-return guard at `internal/desktop/app.go:420` (`if name == "" || name == a.session.Current()`) stays untouched.
- Test run command: `go test ./internal/desktop/` (package), `go test ./internal/...` at the end.

## Review Focus

- Search filtering hides every menu item → contexts must still be listed. (Pinned by the extended `TestMenuSearchFilters`, Task 1.)
- Context list empty but a context open → header shows the name, no CONTEXTS section, no crash. (Pinned by `TestSideHeaderNoList`, Task 1.)
- No context open at all → muted "No context" header, no auto-selection. (Pinned by existing `TestEmptyContextView` — re-run, do not modify.)
- Clicking a non-active context switches and lands on Overview. (Pinned by `TestContextRowClickOpensOverview` + existing `TestSwitchContextRefreshesList`, Task 2.)
- Re-clicking the active context: no watch restart, no navigation. Guard at `app.go:420` must survive; review this line by eye — no test pins the guard.

---

### Task 1: Static header + CONTEXTS section rendering

**Files:**
- Modify: `internal/desktop/shell.go:204-219` (`drawContextHeader`)
- Modify: `internal/desktop/shell.go:233-255` (`drawSide` scroll column — insert contexts block before the menu-section loop)
- Add: `internal/desktop/shell.go` — new `contextRow` method, placed directly after `sideRow` (ends at `shell.go:297`)
- Test: `internal/desktop/app_test.go:444-462` (rewrite `TestSideHeaderSelectsContext`, `TestSideHeaderNoList`), `app_test.go:464-475` (extend `TestMenuSearchFilters`)

**Interfaces:**
- Consumes: `a.contexts []string`, `a.current string`, `sideRowH` (shell.go:28), `IconNamed("server")`, theme tokens `t.TextMuted`, `t.Accent`, `t.SurfacePressed`, `t.SurfaceHover`, `t.Text`.
- Produces: `func (a *App) drawContextHeader(c *ui.Context, t *ui.Theme)` (unchanged signature, simplified body) and `func (a *App) contextRow(c *ui.Context, t *ui.Theme, name string, active bool)` (Task 2 wires the click inside it). Called only from `drawSide`.

- [ ] **Step 1: Write the failing tests**

Rewrite the two header tests in `internal/desktop/app_test.go`:

```go
func TestSideHeaderSelectsContext(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = []string{"dev", "prod"}
	a.current = "dev"
	tt := ui.NewTester(a.view, 1100, 720)
	if _, ok := tt.Find("CONTEXTS"); !ok {
		t.Fatalf("missing contexts section, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("prod"); !ok {
		t.Fatalf("missing context row, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("dev"); !ok {
		t.Fatalf("missing active context, texts %q", tt.Texts())
	}
}

func TestSideHeaderNoList(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = nil
	a.current = "dev"
	tt := ui.NewTester(a.view, 1100, 720)
	if _, ok := tt.Find("dev"); !ok {
		t.Fatalf("missing active context, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("CONTEXTS"); ok {
		t.Fatalf("contexts section drawn without contexts, texts %q", tt.Texts())
	}
}
```

Extend `TestMenuSearchFilters` (add two lines after `a.menuQuery = "virtual"` and one assertion before the final `}`):

```go
	a.contexts = []string{"dev", "prod"}
	a.current = ""
```

```go
	if _, ok := tt.Find("dev"); !ok {
		t.Fatalf("contexts section filtered by menu search, texts %q", tt.Texts())
	}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/desktop/ -run 'TestSideHeader|TestMenuSearchFilters'`
Expected: FAIL — no "CONTEXTS" text exists yet (header still draws the dropdown).

- [ ] **Step 3: Implement in `internal/desktop/shell.go`**

Replace the body of `drawContextHeader` (shell.go:204-219) — the `ui.Select`, the `len(a.contexts) == 0` branch, and the `sel != a.current` switch block all go away:

```go
func (a *App) drawContextHeader(c *ui.Context, t *ui.Theme) {
	if a.current == "" {
		ui.Text(c, "No context").FontSize(13).FontWeight(650).TextColor(t.TextMuted).SingleLine()
		return
	}
	ui.Text(c, a.current).FontSize(13).FontWeight(650).SingleLine().Tooltip(a.current)
}
```

Inside `drawSide`'s scroll column (the `ui.Column` at shell.go:234), insert before the `shown := 0` / menu-section loop:

```go
				if len(a.contexts) > 0 {
					ui.Text(c, "CONTEXTS").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(8, 8, 2, 8)
					for _, name := range a.contexts {
						a.contextRow(c, t, name, name == a.current)
					}
				}
```

Add `contextRow` after `sideRow` (shell.go:297). Section-header text and row geometry copy the menu pattern; deliberately no click wiring yet:

```go
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
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/desktop/`
Expected: PASS — including the untouched `TestEmptyContextView` ("No context" header) and `TestSidebarScrolls`.

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/shell.go internal/desktop/app_test.go
git commit -m "feat(desktop): render kube context list in sidebar"
```

### Task 2: Click a context row to switch

**Files:**
- Modify: `internal/desktop/shell.go` (`contextRow` — add the click branch at the end)
- Test: `internal/desktop/app_test.go` (new test after `TestSwitchContextRefreshesList`, ends at `app_test.go:561`)

**Interfaces:**
- Consumes: `contextRow` from Task 1; `func (a *App) switchContext(name string)` (app.go:419, existing); `func (a *App) openPage(id string)` (existing, used by `pinButton` at shell.go:196-197); `screenID(a.router.Path())` (existing, used by `drawSide` at shell.go:224).

- [ ] **Step 1: Write the failing test**

```go
func TestContextRowClickOpensOverview(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = []string{"dev", "prod"}
	a.current = "dev"
	a.router.Push("/pods")
	tt := ui.NewTester(a.view, 1100, 720)
	if err := tt.Click("prod"); err != nil {
		t.Fatal(err)
	}
	if got := screenID(a.router.Path()); got != "overview" {
		t.Fatalf("context row click landed on %q, want overview", got)
	}
}
```

Note: `cluster.NewSession()` has no kubeconfig, so `switchContext`'s goroutine fails asynchronously and only sets `a.errText` — the router assertion is synchronous and unaffected. This is intentional; the full switch path is already pinned by `TestSwitchContextRefreshesList`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/desktop/ -run TestContextRowClickOpensOverview`
Expected: FAIL — clicking "prod" does nothing, router stays on "pods".

- [ ] **Step 3: Wire the click in `contextRow`**

Append to the end of `contextRow` (only non-active rows are actionable; the active row must not wire a click at all — that is the "re-click does nothing" behavior, backed by `switchContext`'s guard at app.go:420):

```go
	if !active && row.Clicked() {
		a.switchContext(name)
		a.openPage("overview")
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/desktop/` then `go test ./internal/...`
Expected: PASS for both.

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/shell.go internal/desktop/app_test.go
git commit -m "feat(desktop): switch context by clicking a sidebar row"
```
