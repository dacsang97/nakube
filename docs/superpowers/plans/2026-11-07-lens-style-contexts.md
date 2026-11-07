# Lens-Style Context List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the sidebar CONTEXTS section look like Lens's Navigator cluster list: identity color per context, green status dot on the active row, and the list filtered by the sidebar search box.

**Architecture:** Two render-only changes in `internal/desktop`. A small pure helper maps a context name to a fixed 8-color palette via FNV-1a; `contextRow` uses it for inactive-row icons and gains a `t.Success` dot on the active row. The CONTEXTS block in `drawSide` filters its rows by `a.menuQuery` the same way menu sections already do.

**Tech Stack:** Go 1.27, mygo UI (`github.com/egoist/mygo v0.7.4`), stdlib `hash/fnv`.

**Spec:** `docs/superpowers/plans/reports/brainstorm/2026-11-07-lens-style-context-switch.md`

## Global Constraints

- Do NOT touch the `switchContext` guard at `internal/desktop/app.go:420-422`.
- `drawSide`/`contextRow` must read only the snapshot params `current string, contexts []string` (threaded from `view()`, app.go:455-456 by commits 1b6ffe2 + 3348710). Never reintroduce unlocked `a.current`/`a.contexts` reads in the render path.
- Pinned environmental failures, out of scope, do not "fix": `TestCatalogPage` (internal/desktop), `TestContextInfoHidesCredentials` (internal/cluster). A package run passing = only those fail.
- Catalog page and pin buttons keep their navigate-to-overview behavior; the sidebar row click stays switch-only (commit 041a794).
- Colors and the 6×6 dot are not assertable via `ui.Tester` (text-based) — they are pinned by the palette tests plus a manual visual run. Do not invent fake DOM assertions for them.
- Conventional commits, exact messages given per task.

## Review Focus

- Two contexts sharing a color (8 buckets) is expected — the name text stays the primary identifier and the active row keeps its background highlight (spread test pins ≥5 distinct colors over 64 names — Task 1 Step 1).
- Search must match case-insensitively and ignore surrounding whitespace — pinned by the uppercase-query filter test (Task 2 Step 1).
- Empty query must leave the full list untouched — pinned by existing `TestSideHeaderSelectsContext` (no `menuQuery`, rows visible).
- Empty or exotic names (`""`, long ARNs) must not panic or skew the hash — included in the spread/pin test names (Task 1 Step 1).
- Identity stability across releases: the same context name must land on the same palette slot forever — pinned by the exact `contextPalette[4]` pin on `"prod"` (Task 1 Step 1).

---

### Task 1: Context identity color + active status dot

**Files:**
- Create: `internal/desktop/contextcolor.go`
- Test: `internal/desktop/contextcolor_test.go`
- Modify: `internal/desktop/shell.go:295-312` (`contextRow` children block and icon color)

**Interfaces:**
- Consumes: nothing new (uses existing `ui` package, `IconNamed`, `contextRow` locals).
- Produces: `contextColor(name string) ui.Color` and package-level `contextPalette [8]ui.Color` (both unexported, same package — consumed by `contextRow` wiring in this task; no other task consumes them).

- [ ] **Step 1: Write the failing test** — create `internal/desktop/contextcolor_test.go`:

```go
package desktop

import "testing"

func TestContextColorMapsNamesToPalette(t *testing.T) {
	// Identity stability: the pin locks FNV-1a + palette order. If this ever
	// moves, every user's context colors silently change.
	if got := contextColor("prod"); got != contextPalette[4] {
		t.Fatalf("prod left palette slot 4")
	}
	// Spread: 64 generated names must use at least 5 distinct palette
	// colors, so the list reads as Lens's colored cluster list.
	seen := map[ui.Color]bool{}
	names := []string{"", "dev", "prod", "arn:aws:eks:ap-southeast-1:645419842593:cluster/ght-hz"}
	for i := 0; i < 64; i++ {
		names = append(names, fmt.Sprintf("cluster-%02d", i))
	}
	for _, name := range names {
		seen[contextColor(name)] = true
	}
	if len(seen) < 5 {
		t.Fatalf("palette collapsed: %d distinct colors over %d names", len(seen), len(names))
	}
}
```

Add `"fmt"` and `"github.com/egoist/mygo/ui"` to the test imports (the test is in package `desktop`, so it can read `contextPalette`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/desktop -run TestContextColorMapsNamesToPalette -v`
Expected: FAIL — build error `undefined: contextColor` (and `contextPalette`)

- [ ] **Step 3: Implement the helper** — create `internal/desktop/contextcolor.go`:

```go
package desktop

import (
	"hash/fnv"

	"github.com/egoist/mygo/ui"
)

// contextPalette holds the fixed identity colors for kube contexts, in the
// spirit of Lens's colored cluster icons. Mid luminance so they read on both
// light and dark appearances.
var contextPalette = [8]ui.Color{
	ui.Hex("#ef4444"), // red
	ui.Hex("#f97316"), // orange
	ui.Hex("#f59e0b"), // amber
	ui.Hex("#84cc16"), // lime
	ui.Hex("#14b8a6"), // teal
	ui.Hex("#3b82f6"), // blue
	ui.Hex("#8b5cf6"), // violet
	ui.Hex("#ec4899"), // pink
}

// contextColor maps a context name to its identity color. The mapping is a
// plain hash so the same name always lands on the same color.
func contextColor(name string) ui.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	return contextPalette[h.Sum32()%uint32(len(contextPalette))]
}
```

(The pin value: FNV-1a32 of `"prod"` = bucket 4.)

- [ ] **Step 4: Wire the row** — in `internal/desktop/shell.go`, `contextRow` (lines 295-312), replace the icon-color selection and the children block so the function body becomes:

```go
func (a *App) contextRow(c *ui.Context, t *ui.Theme, name string, active bool) {
	row := ui.Row(c).Height(sideRowH).Padding(0, 8).Gap(8).Radius(4).AlignItems(ui.Center)
	icon := contextColor(name)
	if active {
		row.Background(t.SurfacePressed)
		icon = t.Accent
	} else if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Icon(c, IconNamed("server")).Size(16, 16).TextColor(icon).Shrink(0)
		ui.Text(c, name).FontSize(13).SingleLine().MinWidth(0).Grow(1).Tooltip(name)
		if active {
			ui.Box(c).Width(6).Height(6).Radius(3).Background(t.Success).Shrink(0)
		}
	})
	if !active && row.Clicked() {
		a.switchContext(name)
	}
}
```

(Inactive rows get the identity color; the active row keeps accent + highlight and gains the Lens-style green dot. `t.Success` is mygo's semantic green; the click branch from commit 041a794 stays byte-identical.)

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/desktop -run 'TestContextColorMapsNamesToPalette|TestSideHeaderSelectsContext|TestSideHeaderNoList|TestContextRowClickStaysOnPage' -v`
Expected: PASS (all four — text-based assertions are unaffected by color changes)

- [ ] **Step 6: Package check + build**

Run: `go build ./... && go test ./internal/desktop`
Expected: build clean; only the pinned `TestCatalogPage` fails

- [ ] **Step 7: Commit**

```bash
git add internal/desktop/contextcolor.go internal/desktop/contextcolor_test.go internal/desktop/shell.go
git commit -m "feat(desktop): identity color and status dot for context rows"
```

---

### Task 2: Filter CONTEXTS by the sidebar search

**Files:**
- Modify: `internal/desktop/shell.go:225-231` (CONTEXTS block inside `drawSide`)
- Test: `internal/desktop/app_test.go` (extend `TestMenuSearchFilters` tail at :474-486; add `TestContextsFilterBySearch` after `TestSideHeaderNoList`)

**Interfaces:**
- Consumes: `drawSide` snapshot params `contexts []string`, the `a.menuQuery` string bound to the sidebar `TextInput`, and `strings` (already imported by shell.go).
- Produces: nothing.

- [ ] **Step 1: Write the failing tests** — in `internal/desktop/app_test.go`:

Replace the tail of `TestMenuSearchFilters` (the final assertion block, currently `if _, ok := tt.Find("dev"); !ok { t.Fatalf("missing context row, ...") }`) with:

```go
	if _, ok := tt.Find("dev"); ok {
		t.Fatalf("context row survived unrelated filter, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("CONTEXTS"); ok {
		t.Fatalf("contexts header survived unrelated filter, texts %q", tt.Texts())
	}
```

Add after `TestSideHeaderNoList`:

```go
func TestContextsFilterBySearch(t *testing.T) {
	a := New(cluster.NewSession())
	a.menuQuery = "GH"
	a.contexts = []string{"do-ght", "ght-hz", "arn:aws:eks:ap-southeast-1"}
	a.current = ""
	tt := ui.NewTester(a.view, 1100, 720)
	if _, ok := tt.Find("CONTEXTS"); !ok {
		t.Fatalf("missing contexts section, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("do-ght"); !ok {
		t.Fatalf("missing matching context, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("ght-hz"); !ok {
		t.Fatalf("missing matching context, texts %q", tt.Texts())
	}
	if _, ok := tt.Find("arn:aws:eks:ap-southeast-1"); ok {
		t.Fatalf("non-matching context stayed, texts %q", tt.Texts())
	}
}
```

(The uppercase query pins the case-insensitive path; `do-ght`/`ght-hz` both contain `gh`, the ARN does not.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/desktop -run 'TestContextsFilterBySearch|TestMenuSearchFilters' -v`
Expected: FAIL — `TestContextsFilterBySearch`: `non-matching context stayed`; `TestMenuSearchFilters`: `context row survived unrelated filter`

- [ ] **Step 3: Implement the filter** — in `internal/desktop/shell.go`, replace the CONTEXTS block (currently `if len(contexts) > 0 { ... }` with the plain loop) with:

```go
				if len(contexts) > 0 {
					shown := contexts
					if q := strings.ToLower(strings.TrimSpace(a.menuQuery)); q != "" {
						shown = nil
						for _, name := range contexts {
							if strings.Contains(strings.ToLower(name), q) {
								shown = append(shown, name)
							}
						}
					}
					if len(shown) > 0 {
						ui.Text(c, "CONTEXTS").FontSize(11).FontWeight(600).TextColor(t.TextMuted).Padding(8, 8, 2, 8)
						for _, name := range shown {
							a.contextRow(c, t, name, name == current)
						}
					}
				}
```

(Indentation: the block sits inside Scroll > Column, one tab deeper than `drawSide`'s body — match the surrounding lines exactly. The header hides when the query matches nothing, same as a menu section with no hits.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/desktop -run 'TestContextsFilterBySearch|TestMenuSearchFilters|TestSideHeaderSelectsContext|TestSideHeaderNoList' -v`
Expected: PASS (all four)

- [ ] **Step 5: Package check + build**

Run: `go build ./... && go test ./internal/desktop`
Expected: build clean; only the pinned `TestCatalogPage` fails

- [ ] **Step 6: Commit**

```bash
git add internal/desktop/shell.go internal/desktop/app_test.go
git commit -m "feat(desktop): filter context list by sidebar search"
```
