---
phase: 3
title: "Container picker and metrics-server fallback"
status: pending
priority: P3
effort: "1d"
dependencies: [1]
---

# Phase 3: Container picker and metrics-server fallback

## Overview
Container dropdown in Pod metrics (Lens-like) and metrics-server fallback when no Prometheus.

## Requirements
- Functional
  - Pod with >1 container: dropdown "All containers" + each name; selection part of metricsKey; Network tab ignores container (pod-level, note shown).
  - No Prometheus but metrics-server present: CPU/Memory tabs only (Network/FS disabled with tooltip), in-RAM samples every 15s per visible pod, source line "Displaying metrics from metrics-server · since HH:MM". Range dropdown limited to what's collected.
  - Neither → "Metrics not available".
- Non-functional: sampling only for pods whose panel is visible; samples dropped 5m after unseen; no invented values.

## Architecture
- `cluster.PodUsage(ctx, ns, name) (PodUsage, error)` via `MetricsV1beta1().PodMetricses(ns).Get` → per-container CPU/mem; reuse `MetricsAPIMissing`.
- `desktop/metrics.go`: entry.mode = prometheus|metricsServer; metrics-server samples appended to entry ring (cap 7d/15s? → cap 24h, ~5760 pts; downsample to 60 slots via `chart.MinMax`/bucket avg for draw).
- Fallback chosen only when discovery returns `ErrPromNotFound` (not on 403/query error — those surface).

## Related Code Files
- Modify: `internal/cluster/metrics.go` (+ test), `internal/cluster/promql.go` (container filter already param), `internal/views/metrics/{model,view}.go`, `internal/desktop/metrics.go`
- Modify: `README.MD`

## Implementation Steps
1. `PodUsage` + fake metrics clientset test.
2. Container dropdown + key change + tests.
3. Fallback mode, sampling ring, bucketing to slots + tests (gaps when sampler paused).
4. UI states for disabled tabs; README.

## Success Criteria
- [ ] Container switch changes series; "All" equals sum.
- [ ] Cluster without Prometheus shows growing CPU/Mem chart with "since" label.
- [ ] 403 on Prometheus does NOT silently fall back.

## Risk Assessment
- Memory growth from sampling → visible-only + eviction + cap.
- Users confusing short fallback history with broken chart → explicit "since HH:MM".
