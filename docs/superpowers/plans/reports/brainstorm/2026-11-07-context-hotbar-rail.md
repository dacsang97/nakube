# Decision: context list moves into the activity rail (Lens Hotbar)

Date: 2026-11-07
Status: approved (user "ok" after design presented)

## History
- Round 2 built the sidebar CONTEXTS list (identity colors, green active dot, switch-only clicks).
- Round 4 removed the Catalog page, the rail "+" button, and pins (user chose full removal). Rail left with only the Settings button.
- Round 5 (this): user — "để cái sidebar context sang chỗ dấu + kia chứ? để trong sidebar hiện tại làm gì?" — the context list belongs where "+" was (the rail), not in the sidebar.

## Decision (approved design)
- Rail (48px, left): one 32×32 button per context — server icon 20×20 tinted with the context's identity color (`contextColor`), `Tooltip(name)` + `Label(name)` (a11y + Tester-clickable), hover `t.SurfaceHover`, active `t.SurfacePressed` + 6×6 green dot (`t.Success`) pinned to the button's bottom-right corner. Click on non-active = `switchContext(name)` only (no page navigation). List top-aligned in a `ui.Scroll` so many contexts scroll; Settings stays bottom.
- Sidebar: CONTEXTS section (header + rows) deleted. `drawSide` drops its `contexts` param (header with the static active-context name stays).
- Sidebar search no longer filters contexts (nothing left to filter); `menuQuery` is menu-only again.
- `drawActivityBar` signature becomes `(c, t, current string, contexts []string)`; `contextRow` is deleted, replaced by `railContextButton`.
- Untouched: switchContext guard, Bootstrap→ensureCatalog→syncCatalog→startCatalogWatch pipeline, cluster package, identity palette (contextcolor.go).

## Alternatives rejected
- Flyout popover at "+" (round-4 idea) — superseded by full catalog removal; rail button per context is the Lens Hotbar look the user keeps pointing at.
- Letter avatars in the rail (catalog card style) — needs a wider rail; icon-only matches the reference.
- Keep contexts in sidebar AND rail — duplicate surfaces, user explicitly asked why keep it in the sidebar.

## Risks
- Icon-only entries have no visible text → mitigated: `.Label(name)` puts each name into the a11y labels, which ui.Tester.Find/Click match (same pattern as shell.go:110 buttons), so tests click by name without visible text.
- `menuQuery` filter test for contexts retires — acceptable, feature is gone by design.
- Active dot must stay inside the 32×32 button; coordinates (22,22,6×6) verified against `activitySlot`/button size.
