# Context list in sidebar (replaces header dropdown) — Design (short)

Date: 2026-11-06
Supersedes the dropdown part of `2026-11-06-context-dropdown-design.md`.

## Intent & why

The sidebar-header dropdown (`ui.Select`) gives the user nothing usable to
click. Switching context should be manual and direct: a visible list of
contexts inside the left sidebar; click one to switch.

## Goals

- Remove the dropdown from the sidebar header.
- Header becomes a static label: active context name (single-line truncate,
  tooltip with full name), or muted "No context" when none.
- New "Contexts" section at the top of the sidebar list (above the menu
  sections): one row per context, always visible, excluded from the menu
  search filter.
- Active context row highlighted (accent, same treatment as a selected
  `sideRow`); long names truncate to one line, full name via tooltip.
- Clicking a row: `switchContext(name)` + `openPage("overview")` — identical
  to current dropdown/rail-pin behavior. Re-clicking the active context does
  nothing (`switchContext` early-return guard stays).

## Non-goals

- No collapsible context section, no per-context badges/status.
- No changes to `switchContext` internals, catalog/rail pins, MCP/remote.
- No context editing (import/remove kubeconfig).

## Constraints

- Contexts come from cached `a.contexts` (App field); active is `a.current`.
  No queries inside draw (established pattern: `drawSide` reads fields
  directly, no locking in draw).
- Draw functions must not add mutex acquisition.

## Approaches considered

1. **Contexts section in the sidebar list (chosen)** — reuses the existing
   `sideRow` pattern (hover, active highlight, single-line truncate); scales
   with any number of contexts; integrates with the scrolling list.
2. **Always-expanded list in the header block** — rejected: many contexts
   would swallow the entire sidebar; header height is fixed today.
3. **Keep the dropdown plus the list** — rejected: user asked to replace it.

## Left to the builder

- Exact section title text ("Contexts") and row height — reuse `sideRowH`.
- Whether a context row gets an icon (`sideRow` draws one) — suggest a
  cluster/server icon; icon only, no GVR lookup.
