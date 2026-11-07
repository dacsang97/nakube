# Decision — Lens-style context switching in the sidebar

Date: 2026-11-07 · Status: awaiting user confirmation of one open choice

## Outcome

Make the sidebar CONTEXTS section present and behave like Lens's Navigator
cluster list (user's reference screenshot, Lens 2025.x Navigator): each
context gets a stable identity color, the active/connected context shows a
green status dot, and the list is searchable. Switching behavior (click to
switch, stay on current page) was settled in 2026-11-07-context-click-stays
and is kept.

## Repository evidence

- `internal/desktop/shell.go:295-312` — `contextRow`: server icon tinted
  `t.TextMuted` (inactive) / `t.Accent` (active), row highlight
  `t.SurfacePressed`, click → `switchContext(name)` only.
- `internal/desktop/shell.go:225-231` — CONTEXTS section gated on
  `len(contexts) > 0`, currently ignores `a.menuQuery` (menu sections below
  do filter by it).
- `internal/desktop/app.go:455-456` — `view()` already snapshots
  `current` + `contexts` under one `a.mu` hold (race fix, commits 1b6ffe2 +
  3348710); any new row content must consume those snapshot params.
- `mygo` theme (`ui/theme.go`): fixed semantic colors only
  (Accent/Danger/Warning/Success…), but `ui.Hex()` (`ui/color.go:33`) allows
  a fixed custom palette; icons paint via `.TextColor()` (icons.go:10 —
  "The renderer paints them with the text color").
- Precedent for avatar-style elements: catalog card letter avatar
  `internal/views/catalog/view.go:95-102` (single accent color, no per-name
  colors yet).
- Lens reference (docs.lenshq.io Navigator + user screenshot): cluster rows
  show a colored cluster icon (stable per cluster), the connected cluster
  carries a green status dot on the row, long names ellipsize, and clusters
  are searchable (CMD+P quick switcher groups by connection state).
- `switchContext` guard `internal/desktop/app.go:420-422` must stay untouched
  (established constraint from both prior rounds).

## Options considered

| Option | What | Effort | Verdict |
|---|---|---|---|
| A. Lens-style presentation | Per-context identity color (hash of name → fixed 8-hue palette) on the row icon; green `t.Success` dot on the active row's right edge; CONTEXTS rows filtered by the sidebar search box | Small (1 task, no new state, no network) | **Recommended** |
| B. A + Navigator-style expandable tree | Cluster rows expand into resource sub-trees like Lens | Large; duplicates NaKube's existing menu sections per screen | Rejected — the app already has per-cluster navigation menus |
| C. A + real connection states | Probe every context (connected/disconnected dots like Lens) | High: exec-credential per context (EKS/AWS tokens expire), network fan-out in a TUI | Rejected — green dot on the *active* context is the honest subset |
| D. Keep current design | No change | — | Rejected — user explicitly asked to match Lens |

## Recommendation

Option A. The user's own Lens screenshot shows the gaps are
presentation-level: color identity per cluster, green dot on the connected
one, searchable list. All three are render-only changes on top of the
already race-safe snapshot pattern.

Design details:

- Color: FNV-1a hash of the context name → index into a fixed 8-hue palette
  (`ui.Hex` constants, mid-luminance so they read in light and dark).
  Deterministic: same name → same color across launches.
- Active row keeps today's treatment (accent icon + `t.SurfacePressed`
  background) and gains a 6×6 `t.Success` dot pinned at the row's right
  edge (mirrors Lens's green dot on `ght-hz`).
- Filtering: a context row is shown when
  `strings.Contains(strings.ToLower(name), strings.ToLower(a.menuQuery))`,
  matching how menu sections already filter; the section header still shows
  when at least one row survives.

## Risks and mitigations

- Color-only differentiation → the context name remains the primary
  identifier (13px text unchanged); active row keeps background highlight
  and the status dot sits in a fixed position, so nothing relies on hue
  alone.
- Palette readability in light + dark → choose mid-luminance hexes once;
  verify by eye in both appearances (no automated color assertions).
- `a.menuQuery` read inside `drawSide` is the same pre-existing unlocked-read
  class already ledgered as a deferred minor — no new pattern introduced.

## Agreed scope

- `contextRow` (shell.go:295-312): colored icon, active dot, consumed via
  the existing `current`/`contexts` snapshot params.
- CONTEXTS block (shell.go:225-231): apply `a.menuQuery` filter.
- Tests (TDD): deterministic color mapping; dot rendered only when active;
  filter hides non-matching rows; all existing tests stay green.
- Out of scope: connectivity probes, source grouping ("Local Kubeconfigs" vs
  cloud), expandable trees, hotbar/catalog/pin behavior changes (pins and
  catalog keep navigate-to-overview), `switchContext` guard.

## Open decision, if any

Icon treatment — pick one:
1. **Tinted server icon per context color** (recommended: keeps the app's
   icon language; closest analog to Lens's tinted cluster icon).
2. Colored dot *instead of* the icon (more Lens-minimal, loses the icon).

## Planning handoff

Once the open choice is confirmed: superpowers:writing-plans →
`docs/superpowers/plans/2026-11-07-lens-style-contexts.md` →
subagent-driven development (TDD + task review + whole-branch final review),
same pipeline as the two previous rounds.
