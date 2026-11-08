# Global Namespace Selector — Design

Date: 2026-07-11
Status: approved direction, pending plan

## Intent

NaKube watches the whole cluster but offers no way to scope lists to one
namespace. Lens-style: pick a namespace once (or All), and every list
screen filters to it.

## Goals

- A selector (dropdown + quick-switch shortcut) on list screens:
  Overview, Workloads, generic resource lists, Events.
- Namespace choices come from the existing `namespaces` informer cache.
- Filtering happens in view `Build()` calls — the watch stays cluster-wide.
- Per-tab memory of the selected namespace, like other list state.

## Non-goals

- No informer restart or namespace-scoped watch.
- No persistence across app restarts.
- No per-screen separate namespace filters.
- Not applied to Nodes, Namespaces, Argo screens (cluster/app-scoped).

## Anti-goals

- A broken Helm flow: Helm keeps its existing namespace form; selector
  may replace it only if it drops out naturally.

## Approaches considered

1. **Global selector, per-tab state (chosen)** — Lens-like UX, filtering
   in views; low risk, no watch changes.
2. Per-screen namespace filters — more flexible but repeated everywhere,
   no single place to see/change scope; rejected.
3. Namespace-scoped informers — wasteful restarts, no benefit since the
   cache already holds everything; rejected.

## Design

- `App.nsFilter string` ("" = All) mirrored in `tabBody` and swapped on
  tab switch, following the `helmNamespace` pattern.
- Selector UI above list tables; Cmd/Ctrl+P opens a searchable
  quick-switch palette.
- `workloads.Build`, generic builds, and `events.Build` gain a namespace
  parameter; empty string means no filtering.
- Selecting a namespace clears list selection.

## Left to the builder

- Exact shortcut key (avoid collisions with existing bindings).
- Selector visual treatment consistent with the theme chips.
