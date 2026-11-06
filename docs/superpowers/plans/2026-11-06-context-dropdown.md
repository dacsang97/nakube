# Context Dropdown in Sidebar Header — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the static "NaKube" text in the sidebar header with a dropdown that switches the app's k8s context.

**Architecture:** The App already caches context names (`a.contexts`) and the active context (`a.current`), and already has `switchContext(name)` doing the full switch (session.Use → restart watch → refresh metrics → reload list → redraw). The header in `drawSide` renders `ui.Select` from those fields; a changed selection calls `switchContext` + `openPage("overview")`, exactly like the rail buttons do today.

**Tech Stack:** Go, mygo (`github.com/egoist/mygo`) UI toolkit, client-go.

**Spec:** `docs/superpowers/specs/2026-11-06-context-dropdown-design.md`

## Global Constraints

- Draw functions read App fields directly without locking (established pattern: `drawSide`, `drawCatalog`); they must not add mutex acquisition inside draw.
- Long context names render as a single truncated line; the full name stays visible via tooltip.
- When no context is open (`a.current == ""`) the header shows muted "No context" text — never auto-selects an option on first draw.
- Window title in `cmd/nakube/main.go` stays "NaKube"; only the in-app sidebar header changes.
- Test run command: `go test ./internal/...` (repo has no separate test script).

## Review Focus

- Kubeconfig with zero contexts + nothing open: header must read "No context", app must not crash or auto-switch. (Pinned by Task 1 test.)
- Context list empty but a context somehow open: header shows the active name as plain text, not an empty select. (Pinned by Task 1's `TestSideHeaderNoList`.)
- Selecting the already-active context from the dropdown: no watch restart — `switchContext`'s early return must stay the guard. (Pinned by Task 2 test.)

---

### Task 1: Header select rendering and empty-state text

**Files:**
- Modify: `internal/desktop/shell.go:207-213` (header row inside `drawSide`)
- Modify: `internal/desktop/app_test.go:421-435` (`TestEmptyContextView`)
- Test: `internal/desktop/app_test.go` (new test in the same file)

**Interfaces:**
- Consumes: `a.contexts []string`, `a.current string`, `a.session.Current()` (existing App fields/methods).
- Produces: `(a *App) drawContextHeader(c *ui.Context, t *ui.Theme)` — called by `drawSide`; no signature relied on by later tasks beyond this call site.

- [ ] **Step 1: Write the failing tests**

In `internal/desktop/app_test.go`, update `TestEmptyContextView`: replace the `HasText("NaKube")` check with `HasText("No context")` (keep the Overview/Workloads/Pods checks).

Add two new tests below it:

```go
func TestSideHeaderSelectsContext(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = []string{"dev", "prod"}
	a.current = "dev"
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("dev") {
		t.Fatalf("missing active context, texts %q", tt.Texts())
	}
}

func TestSideHeaderNoList(t *testing.T) {
	a := New(cluster.NewSession())
	a.contexts = nil
	a.current = "dev"
	tt := ui.NewTester(a.view, 1100, 720)
	if !tt.HasText("dev") {
		t.Fatalf("missing active context, texts %q", tt.Texts())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/desktop/ -run 'TestEmptyContextView|TestSideHeader'`
Expected: FAIL — "No context" absent (header still draws "NaKube").

- [ ] **Step 3: Implement `drawContextHeader` in `internal/desktop/shell.go`**

Replace the header children (`ui.Text(c, "NaKube").FontSize(13).FontWeight(650).SingleLine()`) with a call to the new method. Body:

- `len(a.contexts) == 0 || a.current == ""` → draw `ui.Text(c, "No context")` muted (`t.TextMuted`, font size 13) — same row height and padding as today.
- otherwise → `sel := a.current; ui.Select(c, &sel, a.contexts).Grow(1).SingleLine()`, give it the header row's remaining width, and if `sel != a.current` call `a.switchContext(sel)` then `a.openPage("overview")`.
- The trigger's visible label is the context name; add `.Tooltip(a.current)`-style tooltip via the Select parts so the full untruncated name is reachable.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/desktop/`
Expected: PASS (whole package, catches sidebar layout regressions).

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/shell.go internal/desktop/app_test.go
git commit -m "feat(desktop): switch kube context from a sidebar header dropdown"
```

### Task 2: Refresh the cached context list after a switch

**Files:**
- Modify: `internal/desktop/app.go:419-436` (`switchContext`)
- Test: `internal/desktop/app_test.go`

**Interfaces:**
- Consumes: `a.loadContexts()` (existing, app.go:257), `a.switchContext(name string)` (existing).
- Produces: no new symbols; behavior contract — after `switchContext` completes, `a.contexts` reflects the session's current context list.

- [ ] **Step 1: Write the failing test**

In `internal/desktop/app_test.go`, add:

```go
func TestSwitchContextRefreshesList(t *testing.T) {
	s := cluster.NewSession()
	a := New(s)
	a.contexts = []string{"stale-a", "stale-b"}
	a.current = "stale-a"
	a.switchContext("stale-b")
	// switchContext runs Use on a goroutine; poll briefly for completion.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		cur := a.current
		n := len(a.contexts)
		a.mu.Unlock()
		if cur == "stale-b" {
			if n == 0 {
				t.Fatalf("context list left stale-empty after switch")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("switch did not complete")
}
```

(This test needs a session whose contexts exist; if `New(cluster.NewSession())` on this machine has no kubeconfig, construct the session the way `internal/cluster/session_test.go:11` does — `KUBECONFIG` env pointing at a temp file with the two-context fixture — and assert the list still contains both names after the switch. The assertion that matters: `loadContexts()` ran, so `a.contexts` is re-read from the session rather than left as the stale slice.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/desktop/ -run TestSwitchContextRefreshesList`
Expected: FAIL or the stale list surviving — `switchContext` does not call `loadContexts()` today.

- [ ] **Step 3: Implement**

In `switchContext`'s goroutine, after the successful `a.session.Use(...)` and before `a.invalidate()`, call `a.loadContexts()` (it locks `a.mu` itself, so call it outside the `a.mu.Lock()` block that sets `a.current`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/app.go internal/desktop/app_test.go
git commit -m "fix(desktop): refresh context list cache after switching context"
```
