# Context dropdown in sidebar header — Design

Date: 2026-11-06

## Intent & why

The sidebar header currently shows a static "NaKube" title. Switching k8s
context today requires opening the catalog/rail. The user wants to switch
context directly: the header becomes a dropdown selection.

## Goals

- Replace the "NaKube" text in the sidebar header with a context dropdown.
- Selecting a different context switches the whole app to it (same behavior
  as existing rail/pin switching: `session.Use`, restart watches, refresh
  metrics, reload list, land on Overview).
- Show the active context name in the dropdown at all times.

## Non-goals

- No new switch mechanism (no modal, no command palette).
- No changes to catalog page, rail pins, or MCP/remote behavior.
- No context editing (import/remove kubeconfig) here.

## Constraints

- The context list comes from `session.ContextInfos()`; it must be cached,
  not queried every frame. Refresh on context switch.
- Long context names (e.g. `arn:aws:eks:...`) must render as a single
  truncated line inside the fixed-height header; full name via tooltip.
- Must work when no context is open yet (empty/placeholder state).

## Approaches considered

1. **`ui.Select` bound to the active context name (chosen)** — simplest;
   mygo provides it; one line of state, change handler calls the existing
   `switchContext`. Trade-off: default select styling, options list is plain
   text.
2. **`ui.MenuButton`** — custom label + menu; more control over styling but
   more code, manual active marking, and manual label updates.
3. **Custom popover list** — rejected: duplicates what `Select` already does.

## Decisions

- Use `ui.Select`; full context name as option label, single-line truncate
  with tooltip for the full name.
- Cache context names on the App; reload after each switch and on first draw.
- On change: `switchContext(name)` + `openPage("overview")` (matches pin
  button behavior).
- Update `app_test.go` header assertions ("NaKube" text check).

## Left to the builder

- Exact placeholder text when no contexts exist (suggest "No context").
- Whether to show context count next to the dropdown (suggest: no).
