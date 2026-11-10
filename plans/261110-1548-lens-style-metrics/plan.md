---
title: "Lens-style metrics panel"
status: pending
created: 2026-11-10
scope: project
blockedBy: []
blocks: []
---

# Lens-style metrics panel

## Overview
"Metrics" block in shared right detail panel, behaving like Lens: Prometheus history via API-server service proxy, tabs CPU/Memory/Network/Filesystem, range 1h/6h/24h/7d, hover rule+diamond+tooltip, KSM request/limit overlay, "Displaying metrics from Prometheus: ns / svc:port". Fallback metrics-server (P3).

Source: [brainstorm report](../reports/261110-1548-brainstorm-lens-style-metrics.md)

## Key decisions
- Transport: `RESTClient` GET `/api/v1/namespaces/{ns}/services/{svc}:{port}/proxy/api/v1/query_range`. 403 → explicit "needs services/proxy" message.
- Discovery: per-context override (Settings) → known service names. Cached per context.
- Step = range/60 (~60 bars). Missing point = gap, never 0.
- Overlay from kube-state-metrics; absent KSM → lines hidden.
- Poll 60s only while panel visible; refetch on range/tab change; keep stale data until fresh.
- No new deps: client-go REST + `encoding/json`; chart via `mygo/ui/chart`.

## Phases

| # | Phase | Status | Depends |
|---|-------|--------|---------|
| 1 | [Prometheus client and Pod metrics](phase-01-prometheus-client-and-pod-metrics.md) | in-progress | — |
| 2 | [Workload and Node metrics](phase-02-workload-and-node-metrics.md) | pending | 1 |
| 3 | [Container picker and metrics-server fallback](phase-03-container-picker-and-metrics-server-fallback.md) | pending | 1 |

## Dependencies
- No overlapping unfinished plans (only `plans/journals`, `plans/reports` exist).
- One PR per phase.

## Success criteria
- Pod on kube-prometheus-stack cluster shows 1h history on open; hover marker; range switch reloads.
- No KSM → bars only. No Prometheus → clear state (P1) / metrics-server fallback (P3).
- `make fmt-check vet test lint` pass; tests need no cluster.

## Validation Log

### Session 1 — 2026-11-10
**Verification Results**
- Claims checked: 12 · Verified: 10 · Failed: 2 · Unverified: 0 · Tier: Standard
- Failures:
  - `chart.Linear` used as curve → actual `chart.CurveLinear` (`mygo/ui/chart/shape.go:23`). Fixed.
  - `Series.Dashed` → `Painter.StrokePath` has no dash (`mygo/ui/path.go:766`); dashes are border-only. Fixed (removed).
- Noted: icons `memory-stick.svg`, `arrow-up-down.svg` absent → already "create if absent".
- Verified: `ui.Select`, `ui.TextInput`, `ui.Field`, `chart.Select/RuleX/Point/Anchor`, `Linear.Nice(int)`, `kit.Legend`, `ui.NewTester`, `App.invalidate`, context reset `app.go:272-282`, `UserConfigDir` persistence pattern (`theme.go`).

**Decisions**
| Q | Answer |
|---|---|
| Request/limit line style | solid, distinct colors |
| Metrics placement in Pod panel | top, above details (Lens) |
| Tab/range memory | reset to CPU/1h on each resource change |
| CPU unit | auto: `<1` → `m`, `≥1` → cores |

### Whole-Plan Consistency Sweep
- Searched for `Dashed`, `chart.Linear)`, persistence wording: no stale refs remain. P2/P3 inherit chart/state from P1; no contradictions. Unresolved: 0.
