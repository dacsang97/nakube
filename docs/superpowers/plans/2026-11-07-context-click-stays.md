# Context Row Click Stays On Page — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clicking a context row in the sidebar switches context but no longer navigates to Overview — the app stays on the page the user is viewing.

**Architecture:** One-line behavior change in `contextRow` (drop `a.openPage("overview")` from the click branch) plus a rewritten test that pins the new behavior with a real kubeconfig fixture and hermetic state isolation. The switch stays asynchronous, so the test polls for `a.current` to change.

**Tech Stack:** Go, mygo UI tester (`ui.NewTester`), existing `cluster` session APIs. No new dependencies.

**Spec:** Conversation-approved design (round 2, 2026-11-07): (1) sidebar context click → switch only, no page navigation; (2) no code change — the sidebar already lists merged contexts (active kubeconfig + every `.yaml`/`.yml` in `~/.kube` via `SyncKubeDir` + watcher). Prior spec: `docs/superpowers/specs/2026-11-06-context-list-sidebar-design.md`.

## Global Constraints

- Do NOT touch the `switchContext` guard at `internal/desktop/app.go:420-422` (`if name == "" || name == a.session.Current() { return }`).
- Catalog card click (`OnOpen`, rail.go:40-43) and pin button (`pinButton`, shell.go:185-202) KEEP their navigate-to-overview behavior — only the sidebar rows change.
- Tests must be hermetic: `t.Setenv("HOME", dir)`, `t.Setenv("KUBECONFIG", <fixture in t.TempDir()>)`, `s.SetStatePath(…)` — no writes to real `~/.config/nakube/context`, no real kubeconfig leakage (same isolation pattern as commit 13dc7d1 and neighbor `TestSwitchContextRefreshesList`).
- Known environmental failure `TestCatalogPage` (pinned, fails on this machine with real `~/.kube` present) is expected and out of scope — do not chase it.
- Conventional commits, scope `desktop`.

## Review Focus

- Double-clicking a context row while a switch is in flight → duplicate `switchContext` goroutines are harmless (second is a no-op or re-applies same name); app must not crash.
- Clicking the ACTIVE context row → no-op (active rows wire no click); unchanged from round 1.
- Switch failure (e.g. context removed from kubeconfig mid-session) → test asserts success path only; failure path self-heals via `loadContexts` — no test added, just don't break it.
- Catalog card and pin button navigation → still go to Overview; existing tests untouched.

---

### Task 2: Read sidebar context state from the render snapshot (production race fix)

**Files:**
- Modify: `internal/desktop/shell.go:204-229` (`drawContextHeader`, `drawSide` signatures + `a.current` reads)
- Modify: `internal/desktop/app.go:467` (pass `current` snapshot into `drawSide`)

**Interfaces:**
- Consumes: the `current := a.current` snapshot already taken under `a.mu` in `view()` (app.go:458) and passed to `drawActivityBar` (app.go:466) — Task 2 routes the same value into `drawSide`.
- Produces: none for later tasks.

**Why:** `drawSide` reads `a.current` unlocked on the render path (shell.go:205 header empty-check, shell.go:209 label text, shell.go:229 active flag) while `switchContext`'s goroutine writes it under `a.mu` (app.go:429). Reproduced as a `WARNING: DATA RACE` by `go test -race` with the new switch test; repo CI runs `-race` (ci.yml:78, Makefile:4). Fix mirrors the existing `drawActivityBar(c, t, current)` pattern — same value, now consistent.

- [ ] **Step 1: Thread the snapshot through**

- `drawContextHeader(c *ui.Context, t *ui.Theme, current string)` — replace both `a.current` reads (shell.go:205, :209) with `current`.
- `drawSide(c *ui.Context, t *ui.Theme, current string)` — call `a.drawContextHeader(c, t, current)` (shell.go:219) and use `current` for the active flag (shell.go:229).
- app.go:467: `a.drawSide(c, t, current)`.

**Amendment (controller ruling after Task 2's first pass):** verification surfaced the identical race class on `a.contexts` — written under `a.mu` by `loadContexts` (app.go:261, via switchContext goroutine app.go:432), read unlocked on the same render path at shell.go:226 (`len(a.contexts)`) and :228 (`range a.contexts`). Extend Step 1 with the `contexts` snapshot: in `view()` add `contexts := a.contexts` beside `current` (still under the one `a.mu` hold), pass `a.drawSide(c, t, current, contexts)`, and let the shell.go:226 gate + :228 range use `contexts`. Other unlocked reads (`a.menuQuery`, `a.menuItems`, …) stay out of scope. Separate commit: `fix(desktop): snapshot sidebar context list for render`.
- Do NOT touch other unlocked field reads in `drawSide` (search box, menu sections) — out of scope.

- [ ] **Step 2: Verify**

Run: `go build ./... && go test ./internal/desktop` (pinned `TestCatalogPage` may fail)
Then: `go test -race -run 'TestContextRowClickStaysOnPage|TestSwitchContextRefreshesList' -count=20 ./internal/desktop`
Expected: build clean; package unchanged; 20/20 race-free — three consecutive runs (the a.current race was eliminated in commit 1b6ffe2; the amendment eliminates the a.contexts race; all three runs must be clean now).

- [ ] **Step 3: Commit**

```bash
git add internal/desktop/shell.go internal/desktop/app.go
git commit -m "fix(desktop): read sidebar context state from render snapshot"
```

---

### Task 1: Sidebar context click switches without navigating

**Files:**
- Modify: `internal/desktop/shell.go:308-311` (click branch in `contextRow`)
- Modify: `internal/desktop/app_test.go:576-594` (`TestContextRowClickOpensOverview` → renamed/rewritten)

**Interfaces:**
- Consumes: `a.switchContext(name string)` (app.go:419), `a.router.Path()` (mygo), `ui.NewTester(a.view, 1100, 720)` + `tt.Click("prod")` (existing test pattern), `cluster.NewSession()` + `s.SetStatePath(path)`.
- Produces: none for later tasks (single-task plan).

- [ ] **Step 1: Rewrite the test**

Replace `TestContextRowClickOpensOverview` (app_test.go:576-594) with `TestContextRowClickStaysOnPage` — same hermetic setup (HOME/KUBECONFIG/SetStatePath) but a real kubeconfig fixture so the switch's `Use` succeeds, and router-stays assertion instead of screen-change:

```go
func TestContextRowClickStaysOnPage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	kubeconfig := filepath.Join(dir, "kubeconfig")
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
  name: dev
- context:
    cluster: c
    user: u
  name: prod
current-context: dev
users:
- name: u
  user:
    token: test-token-not-for-logs
`
	if err := os.WriteFile(kubeconfig, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig) // wire fixture into loading rules (hermetic: file lives in t.TempDir)
	s := cluster.NewSession()
	s.SetStatePath(filepath.Join(dir, "state", "context"))
	a := New(s)
	a.contexts = []string{"dev", "prod"}
	a.current = "dev"
	a.router.Push("/pods")
	tt := ui.NewTester(a.view, 1100, 720)
	tt.Click("prod")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && a.current != "prod" {
		time.Sleep(10 * time.Millisecond)
	}
	if a.current != "prod" {
		t.Fatalf("context did not switch, current=%q", a.current)
	}
	if got := a.router.Path(); got != "/pods" {
		t.Fatalf("router moved to %q, want /pods", got)
	}
}
```

(Keep existing imports; drop `screenID` use only if now unused elsewhere in the file.)

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./internal/desktop -run TestContextRowClickStaysOnPage -v`
Expected: FAIL with `router moved to "/overview", want /pods`

- [ ] **Step 3: Remove the navigation from the click branch**

In `internal/desktop/shell.go:308-311`, change:

```go
if !active && row.Clicked() {
    a.switchContext(name)
    a.openPage("overview")
}
```

to:

```go
if !active && row.Clicked() {
    a.switchContext(name)
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run: `go test ./internal/desktop -run TestContextRowClickStaysOnPage -v`
Expected: PASS

- [ ] **Step 5: Run the package**

Run: `go test ./internal/desktop`
Expected: all pass except the pinned environmental `TestCatalogPage`; `go build ./...` clean.

- [ ] **Step 6: Commit**

```bash
git add internal/desktop/shell.go internal/desktop/app_test.go
git commit -m "feat(desktop): keep current page when switching context from sidebar"
```

- [ ] **Step 7: Rebuild and relaunch the local build for the user's manual check**

```bash
pkill -f /tmp/nakube-feature || true
go build -o /tmp/nakube-feature ./cmd/nakube
nohup /tmp/nakube-feature >/tmp/nakube-feature.log 2>&1 & echo "PID $!"
```

Expected: process starts; `/tmp/nakube-feature.log` stays clean.
