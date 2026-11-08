# Global Namespace Selector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A per-tab namespace selector ("All" or one namespace) that filters Workloads, generic resource lists, and Events, Lens-style.

**Architecture:** Filtering happens in view `Build()` calls and row-building helpers; the cluster-wide informer watch is untouched. State lives on `App` and is mirrored in `tabBody`, following the existing `helmNamespace` pattern, so each tab remembers its own namespace.

**Tech Stack:** Go, mygo UI toolkit, existing informer caches.

**Spec:** `docs/superpowers/specs/2026-07-11-namespace-selector-design.md`

## Global Constraints

- `nsFilter == ""` means All namespaces; never use a sentinel other than "".
- No informer restart, no namespace-scoped watch, no persistence across restarts.
- Selector not applied to: Nodes, Namespaces, Argo screens, Helm form.
- All UI text and code comments in English.
- Checks per stack: `make fmt-check && make vet && make test && make lint` before done.

## Review Focus

- Namespace with zero matching resources: list shows empty table with correct Total, not an error. (Pinned: Task 2 test.)
- `nsFilter` set to All ("") restores full unfiltered lists everywhere. (Pinned: Task 2, Task 3, Task 4 tests.)
- Tab switch preserves each tab's `nsFilter`; two tabs on the same screen show different filters. (Pinned: Task 1 test.)
- Selecting a namespace that is Terminating still filters (selector lists every namespace from the informer cache). (Pinned: Task 5 test.)
- Deleted namespace remaining in `nsFilter`: lists go empty but app must not crash. (Pinned: Task 2 test.)

---

### Task 1: `nsFilter` state and per-tab persistence

**Files:**
- Modify: `internal/desktop/app.go` (~line 110 struct, ~line 189 `New`)
- Modify: `internal/desktop/tabstate.go` (`tabBody` struct, `newTabBody`, the swap functions near lines 196 and 255)
- Test: `internal/desktop/tabs_test.go`

**Interfaces:**
- Produces: `App.nsFilter string` field; saved/restored in `tabBody` swap so tab switches preserve it. Later tasks read `a.nsFilter` and set it.

- [ ] **Step 1: Write the failing test**

In `internal/desktop/tabs_test.go`, add a test `TestTabSwitchKeepsNamespaceFilter`: build an `App` via the existing helpers in that file, set `a.nsFilter = "kube-system"`, save the tab body (call the same save path `a.activate`/`showScreen` uses — mirror what an existing test does for `helmNamespace`), switch to another tab and back, assert `a.nsFilter == "kube-system"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/desktop/ -run TestTabSwitchKeepsNamespaceFilter -v`
Expected: FAIL (field not saved across switch)

- [ ] **Step 3: Implement**

- Add `nsFilter string` to `App` struct and `tabBody` struct.
- In `newTabBody()` leave it `""` (All).
- In the save function (near `tabstate.go:196`, the `b.helmNamespace = a.helmNamespace` block) add `b.nsFilter = a.nsFilter`; in the restore function (near line 255) add `a.nsFilter = b.nsFilter`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/desktop/ -run TestTabSwitchKeepsNamespaceFilter -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/app.go internal/desktop/tabstate.go internal/desktop/tabs_test.go
git commit -m "feat: per-tab namespace filter state"
```

### Task 2: Filter Workloads by namespace

**Files:**
- Modify: `internal/views/workloads/model.go` (`Build`, ~line 113)
- Modify: `internal/desktop/browse.go:609` (call site)
- Test: `internal/views/workloads/model_test.go`

**Interfaces:**
- Produces: `workloads.Build(objects []runtime.Object, query string, filter int, sortBy string, descending bool, namespace string) Page` — new final param; `""` means no filtering. Rows are excluded by `row.Namespace != namespace` before counting; `Page.Total`, `Kinds` summaries, and health counters reflect only matching objects.

- [ ] **Step 1: Write the failing tests**

In `internal/views/workloads/model_test.go`, add:
- `TestBuildNamespaceFilter`: two Deployments in different namespaces, `Build(..., "ns-a")` returns 1 row, `Total == 1`, health counters count only the matching one.
- `TestBuildNamespaceAll`: same objects, `namespace == ""` returns both rows.
- `TestBuildNamespaceMissing`: `namespace == "gone"` returns 0 rows, `Total == 0`, no panic.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/views/workloads/ -run 'TestBuildNamespace' -v`
Expected: FAIL (compile error: too few arguments)

- [ ] **Step 3: Implement**

Change `Build` signature to add the `namespace string` param. Inside the object loop, after `normalize`, `continue` when `namespace != "" && row.Namespace != namespace`, before incrementing any counters. Update the call site in `browse.go:609` to pass `a.nsFilter`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/views/workloads/ ./internal/desktop/ -v`
Expected: PASS (all existing tests updated for the new signature if the compiler flags them)

- [ ] **Step 5: Commit**

```bash
git add internal/views/workloads/model.go internal/views/workloads/model_test.go internal/desktop/browse.go
git commit -m "feat: filter workloads by namespace"
```

### Task 3: Filter Events and generic lists by namespace

**Files:**
- Modify: `internal/views/events/model.go` (`Build`, ~line 56)
- Modify: `internal/views/events/view_test.go` / `internal/views/events/model_test.go` (whichever holds Build tests)
- Modify: `internal/desktop/inspect.go` (events call site, ~line 110)
- Modify: `internal/desktop/browse.go` (`fetchList` ~line 461, `genericRows` ~line 491, `istioRows` ~line 517)
- Test: `internal/desktop/browse_test.go`, events tests

**Interfaces:**
- Produces: `events.Build(items []*corev1.Event, query, column string, desc bool, now time.Time, namespace string) Page` — `""` means all; filters `item.Namespace` before counting; `Total` counts pre-search matches.
- Produces: generic list rows filtered to `a.nsFilter` inside `fetchList`, `genericRows`, and `istioRows` — implement one helper `filterListRows(rows []listRow) []listRow` on `App` in `browse.go` that returns rows unchanged when `a.nsFilter == ""`, else keeps rows with `Namespace == a.nsFilter`; apply it to the returned `[]listRow` in all three functions.

- [ ] **Step 1: Write the failing tests**

- Events test: two events in different namespaces; `Build(..., "ns-a")` returns 1 row and `Total == 1`; `""` returns both. Mirror the existing Build test's fixture style.
- `browse_test.go` test `TestFilterListRows`: construct an `App` with `nsFilter = "ns-a"` and two `listRow`s with different `Namespace` fields; assert one row survives; repeat with `nsFilter == ""` and assert both survive.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/views/events/ ./internal/desktop/ -run 'Namespace|FilterListRows' -v`
Expected: FAIL

- [ ] **Step 3: Implement**

- Add the `namespace` param to `events.Build`; filter in the item loop before `total++`.
- Update the call site in `inspect.go` to pass `a.nsFilter`.
- Add `filterListRows` on `App` and call it in `fetchList`, `genericRows`, `istioRows` return paths.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/views/events/ ./internal/desktop/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/views/events/ internal/desktop/inspect.go internal/desktop/browse.go internal/desktop/browse_test.go
git commit -m "feat: filter events and generic lists by namespace"
```

### Task 4: Namespace selector UI on list screens

**Files:**
- Modify: `internal/desktop/browse.go` (`drawWorkloads` ~line 601 and the shared list-screen drawing path that `fetchList` feeds)
- Modify: `internal/desktop/inspect.go` (events drawing path, ~line 110 area)
- Test: `internal/desktop/browse_test.go`, `internal/desktop/generic_test.go`

**Interfaces:**
- Consumes: `App.nsFilter`, `App.namespaces` (`map[string]*corev1.Namespace`, already populated by the informer).
- Produces: `func (a *App) drawNamespaceSelector(c *ui.Context, t *ui.Theme)` in `browse.go` — a horizontal chip row: an "All" chip plus one chip per namespace (sorted by name, capped display like other label clipping). Clicking a chip sets `a.nsFilter` to that namespace (or `""` for All), resets the list selection (`a.row = -1`), and triggers a redraw through the existing `a.window` coalescing path used by other click handlers (mirror how the filter chips in `workloads.Draw` update `a.listFilter`). Wire it above the table in `drawWorkloads`, the generic list drawing path, and the events drawing path.

- [ ] **Step 1: Write the failing test**

In `browse_test.go`, add `TestSelectNamespaceResetsRow`: build an App with namespaces `{"default", "kube-system"}` in `a.namespaces`, set `a.row = 3`, call the click handler that sets the filter (extract the set logic into a small method `setNamespaceFilter(ns string)` so the test can call it directly), assert `a.nsFilter == ns` and `a.row == -1`; then `setNamespaceFilter("")` and assert `a.nsFilter == ""`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/desktop/ -run TestSelectNamespaceResetsRow -v`
Expected: FAIL (method undefined)

- [ ] **Step 3: Implement**

- Add `setNamespaceFilter(ns string)` on `App` (sets `a.nsFilter`, resets `a.row` to -1, coalesces a redraw).
- Add `drawNamespaceSelector` per the interface above; render with the theme's chip styles the way `workloads.Draw` renders its filter chips (active chip uses the accent/selected tone).
- Insert the selector row above the table in the Workloads drawing path, the generic list path, and the Events path. Skip the selector on screens where it does not apply (Nodes, Namespaces, Argo, Helm).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/desktop/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/browse.go internal/desktop/inspect.go internal/desktop/browse_test.go
git commit -m "feat: namespace selector chips on list screens"
```

### Task 5: Quick-switch shortcut

**Files:**
- Modify: `internal/desktop/tabbar.go` (`tabKeys`, ~line 297)
- Modify: `internal/desktop/browse.go` or a new `internal/desktop/nspalette.go` (the palette overlay)
- Test: `internal/desktop/tabs_test.go` or a new `internal/desktop/nspalette_test.go`

**Interfaces:**
- Consumes: `setNamespaceFilter`, `a.namespaces`.
- Produces: shortcut Cmd/Ctrl+P toggles a lightweight searchable palette overlay: a text field plus the sorted namespace list filtered by the typed prefix (`kit.Match`), Enter applies `setNamespaceFilter` to the highlighted entry, Esc closes. State fields `nsPaletteOpen bool` and `nsPaletteQuery string` live on `App` (not `tabBody`).

- [ ] **Step 1: Write the failing test**

Add `TestNamespacePaletteFilter`: App with namespaces `{"default", "kube-system", "monitoring"}`; with `nsPaletteQuery = "ku"`, the palette's filtered list contains exactly `{"kube-system"}`. Extract the filter into `func (a *App) namespacePaletteItems() []string` (sorted, query-filtered, empty query returns all) and test that directly. Also assert `setNamespaceFilter("monitoring")` closes the palette (`nsPaletteOpen == false`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/desktop/ -run TestNamespacePaletteFilter -v`
Expected: FAIL

- [ ] **Step 3: Implement**

- Add the two state fields; `setNamespaceFilter` sets `nsPaletteOpen = false`.
- Add `namespacePaletteItems`.
- Register the shortcut in `tabKeys`: `case e.Shortcut(ui.Cmd, ui.KeyP): a.nsPaletteOpen = !a.nsPaletteOpen` (Ctrl+P off darwin). Draw the palette as an overlay reusing the pattern of an existing overlay (e.g. the ask/action prompt) when `nsPaletteOpen`; keystrokes route through the existing overlay text-input path.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/desktop/ -v`
Expected: PASS

- [ ] **Step 5: Full checks and commit**

Run: `make fmt-check && make vet && make test && make lint`
Expected: all PASS

```bash
git add internal/desktop/
git commit -m "feat: Cmd+P namespace quick-switch palette"
```

### Task 6: README note

**Files:**
- Modify: `README.MD` (keyboard table, ~line 30)

**Interfaces:** none.

- [ ] **Step 1: Add a row** to the shortcut table: Namespace palette | Command-P | Ctrl-P.

- [ ] **Step 2: Commit**

```bash
git add README.MD
git commit -m "docs: namespace palette shortcut"
```
