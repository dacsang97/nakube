# Decision — remove Catalog page + rail "+" entirely (2026-11-07, superseded flyout idea)

> History: user first floated a flyout at "+" (earlier revision of this file), then decided: the sidebar CONTEXTS list already shows every context from kubeconfig, so the "+" button and Catalog page go away completely. Chosen: Option 1 (full removal incl. pins); Add file/Paste NOT relocated to Settings.

## Outcome
Delete the Catalog UI surface: rail "+" button, Catalog page + route, pins, and the internal/views/catalog package. The rail keeps only ⚙ Settings. The sidebar CONTEXTS section stays the single place to switch contexts. The cluster-side sync pipeline is untouched: ~/.kube auto-sync, watcher, and previously persisted catalog.json "files" keep loading at boot, so all kubeconfig contexts still appear in the sidebar.

## Repository evidence
- mygo v0.7.4 has native primitives: `ui.Popover(c, anchor, open *bool, fn)` (themed panel below anchor, outside-click/Escape dismiss, auto-flip above when no room) and `ui.PopoverBase` (unstyled panel we style ourselves); `Element.Dismissed()` reports dismissal (ui/api_gen.go:1519-1575, widgets.go corePopover).
- Rail: `drawActivityBar` (internal/desktop/shell.go:139-160), width `activityBarW = 48` (shell.go:22); "+" = `activityButton(..., "plus", "Catalog", ...)` → `togglePage("/catalog")` (shell.go:148-151); pins render above it.
- Sidebar CONTEXTS block lives in `drawSide` (shell.go:227-240 post-Task-2) using snapshot params `contexts/current` from `view()` (app.go:455-456) + search filter on `a.menuQuery`.
- Catalog page (rail.go/view.go): card grid of contexts, Add file / Add pasted kubeconfig, pins management — currently the "+" button's only entry point.
- contextRow pieces reusable: identity color `contextColor(name)` (contextcolor.go), active dot t.Success, click → `switchContext(name)` only (041a794).

## Options considered
- **A (earlier revision): flyout từ "+"** — superseded by user's final call.
- **1. Full removal (CHOSEN)** — "+", Catalog page/route, pins, internal/views/catalog all deleted; rail = ⚙ only; Add file/Paste dropped (user accepts; ~/.kube sync + watcher remain); persisted catalog.json files still honored for backward compat.
- **2. Removal + Settings relocation** — rejected by user ("").

## Recommendation
Option 1. Removals pinned in the plan: drawCatalog & page-only helpers, route case, closeCatalog, pinButton, fields catalogPins/catalogSynced/addPath/paste, catalogState.Pins (json ignores unknown keys → old files parse), editor/strings/time/utf8 imports in rail.go, TestCatalogPage, plus-icon assertion in TestSidebarIcons. Kept: ensureCatalog/applyCatalogFiles/addLoadedPath/syncCatalog/startCatalogWatch/readCatalog/catalogFile/catalogDir/sessionState + cluster package entirely (ImportKubeconfig/ContextInfos stay as cluster API with their own tests). drawActivityBar drops `current` param, layout simplifies to Spacer + settings button (ui.Spacer exists, mygo api_gen.go:1994). New tests: TestCatalogRouteRemoved (route → "Not found") + TestEnsureCatalogKeepsPersistedFiles (old catalog.json with pins+files still loads file contexts).

## Risks and mitigations
- Over-pruning breaks the context pipeline → keep-list pinned function-by-function in the plan; reviewer verifies `grep -rn 'SyncKubeDir\|applyCatalogFiles\|startCatalogWatch'` still wired in Bootstrap (app.go:237-241).
- Old catalog.json with "pins" key must still parse → json.Unmarshal ignores unknown fields; regression test TestEnsureCatalogKeepsPersistedFiles.
- Dead imports/fields (Go errors only on imports) → explicit removal list in plan; `go vet` + build catch imports.
- Deleting TestCatalogPage removes the pinned env failure → desktop package expected fully green.

## Agreed scope
Remove: Catalog page/route, "+" button, pins, views/catalog pkg, page-only helpers, fields. Keep: sidebar CONTEXTS (unchanged), cluster catalog machinery, ~/.kube watcher, persisted-files loading, Settings page. Out of scope: Settings relocation of add-source, flyout, connectivity probes.

## Open decision, if any
None — user chose Option 1 (m00748: "1").

## Planning handoff
superpowers:writing-plans → docs/superpowers/plans/2026-11-07-remove-catalog-rail.md → SDD execution.
