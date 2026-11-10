---
phase: 1
title: "Prometheus client and Pod metrics"
status: in-progress
priority: P1
effort: "2d"
dependencies: []
---

# Phase 1: Prometheus client and Pod metrics

## Overview
Prometheus discovery + range query over API-server proxy, PromQL builders, shared Lens-style chart, Metrics block in Pod detail panel, Settings override. Foundation for P2/P3.

## Requirements
- Functional
  - Auto-discover Prometheus; per-context override `ns/svc:port` in Settings.
  - Pod panel: tabs CPU, Memory, Network, Filesystem; range 1h/6h/24h/7d (default 1h).
  - Bars = primary series; lines = secondary (requests/limits, transmit, reads/writes).
  - Hover/arrow keys: orange vertical rule + diamond marker + tooltip (time + every series value).
  - Source line "Displaying metrics from Prometheus: ns / svc:port".
  - States: loading skeleton, discovering, not found, 403 (services/proxy), query error, no data in range.
- Non-functional
  - No new module deps. Never invent values: missing point = gap.
  - Fetch off UI goroutine; poll 60s only while block visible; ≤ 4 queries/tab load, run in parallel.
  - Tests need no cluster/display.

## Architecture
```
desktop (App) ──metricsKey──▶ metricsCache ──goroutine──▶ cluster.Prometheus
     │                                                   ├─ Discover(ctx, override)
     │                                                   └─ QueryRange(ctx, src, q, start, end, step)
     └─ draw: kit.MetricsChart(c, t, view)   ◀── []kit.Series (bars + lines)
```

### `internal/cluster/prometheus.go`
```go
type PromSource struct{ Namespace, Service string; Port int }      // String(): "ns/svc:port"
type PromPoint struct{ At time.Time; Value float64; OK bool }
type PromSeries struct{ Name string; Points []PromPoint }

var ErrPromNotFound = errors.New("prometheus not found")
func PromForbidden(err error) bool // apierrors.IsForbidden

func (s *Session) DiscoverPrometheus(ctx, override string) (PromSource, error)
func (s *Session) QueryRange(ctx, src PromSource, q string, start, end time.Time, step time.Duration) ([]PromSeries, error)
func Step(rng time.Duration) time.Duration // max(60s, rng/60)
```
- Candidates (in order): override → `monitoring/prometheus-operated:9090` → any ns `kube-prometheus-stack-prometheus:9090` → `prometheus-server:80` → `lens-metrics/prometheus:80`. Probe = list services by name via clientset (cache informer not needed), then `GET .../proxy/-/ready` (or `/api/v1/query?query=1`).
- Request: `cs.CoreV1().RESTClient().Get().AbsPath("/api/v1/namespaces", ns, "services", svc+":"+port, "proxy", "api/v1/query_range").Param(...)`.
- Parse Prometheus matrix JSON (`status`, `data.result[].values[[ts,"v"]]`); align to step grid `[start, end]` so gaps = OK false; `NaN`/`+Inf` → gap.
- Cache discovered source per context in Session; reset on context switch.

### `internal/cluster/promql.go` (pure)
```go
type MetricQuery struct{ Name string; Expr string; Line bool } // Line false = bars
func PodQueries(tab, ns string, pods []string, container string) []MetricQuery
```
- Pod selector `namespace="ns", pod=~"^(a|b)$"` with `regexp.QuoteMeta`; `container!="", container!="POD"`, `image!=""` (cAdvisor dedupe).
- CPU: `sum(rate(container_cpu_usage_seconds_total{sel}[{rate}]))`; requests/limits `sum(kube_pod_container_resource_requests{sel,resource="cpu"})` / `..._limits`.
- Memory: `container_memory_working_set_bytes`; KSM `resource="memory"`.
- Network: `rate(container_network_receive_bytes_total{namespace,pod=~}[r])`, transmit (no container label filter — pod-level).
- Filesystem: `container_fs_usage_bytes`, `rate(container_fs_reads_bytes_total[r])`, writes.
- Rate window = max(2×step, 1m) as string.

### `internal/ui/metrics.go`
```go
type Series struct{ Name string; Color ui.Color; Points []Sample; Line bool } // solid lines only (mygo StrokePath has no dash); requests=Requests green, limits=Limits blue
<!-- Updated: Validation Session 1 - dropped Dashed (unsupported) -->


func MetricsChart(c, t, start, end time.Time, series []Series, format func(float64) string, hover *int)
```
- Reuse `kit.Sample`; refactor `Bars` body into shared plot helper (DRY) — Overview keeps calling `Bars`.
- Y: `chart.NewLinear(0, hi).Nice(4)` ticks on right; X: time ticks via existing `clock`.
- `chart.Select(c, area, hover, xs, describe)`; when `*hover >= 0`: `RuleX` orange (`#f5a623`-ish, add to `Series` palette as `Cursor`), `Point` 8×8 rotated-square look (Radius 1 + border) at primary value, tooltip via `chart.Anchor`.
- Lines: `chart.Line(r, xs, ys, chart.CurveLinear)` stroked via `Painter.StrokePath`; gaps → NaN ys (shape skips undefined). <!-- Updated: Validation Session 1 - CurveLinear name -->
- CPU format: <1 core → `"350m"`, ≥1 → `"1.2"` cores. Memory/Network/FS: binary bytes (`Ki/Mi/Gi`, `/s` for rates).
- Legend via existing `kit.Legend`.

### `internal/views/metrics/` (new view pkg, matches views/* pattern)
- `model.go`: `Tab` enum, `Ranges()` (1h/6h/24h/7d), `Key`, `View{Source, Tab, Range, Series, State, Err, FetchedAt}`; formatters (cores/millicores, bytes, B/s).
- `view.go`: `Draw(c, t, v View, tab *string, rng *string, hover *int)` — toolbar (range `ui.Select`/dropdown, 4 icon toggles using `desktop/icons` cpu, memory-stick, arrow-up-down, hard-drive), source line, chart, states.

### `internal/desktop/metrics.go`
- `App.metrics map[metricsKey]*metricsEntry` guarded by `a.mu`; `ensureMetrics(key)` starts fetch when missing/stale(>60s) and none in flight; result → store + `a.invalidate()`.
- Visibility: drawing the block marks `entry.seen = now`; single ticker goroutine (60s) refetches entries seen in last 2s. Drop entries not seen for 5m.
- Context switch (`app.go:277` reset) clears cache.
- Pod panel: in `drawWorkloads` draw closure (`browse.go`), when `item.Kind == "Pod"` draw Metrics block FIRST, then `workloads.Detail` (Lens order). <!-- Updated: Validation Session 1 - placement top -->
- Tab/range state: per open panel; reset to CPU/1h when selected resource changes (`selection.Changed`). Not persisted. <!-- Updated: Validation Session 1 -->
- Range dropdown: `ui.Select(c, &rng, []string{"1h","6h","24h","7d"})`.

### Settings override
- `internal/desktop/promsettings.go`: file `UserConfigDir()/nakube/prometheus.json` map `context → "ns/svc:port"`, perms 0600, same pattern as `theme.go`.
- `views/settings/view.go`: `ui.Field` + `ui.TextInput` "Prometheus service" for current context, placeholder "auto", validate `^[a-z0-9-]+/[a-z0-9-.]+:\d+$`; save clears discovery cache.

## Related Code Files
- Create: `internal/cluster/prometheus.go`, `prometheus_test.go`, `promql.go`, `promql_test.go`, `testdata/prom/matrix.json`, `testdata/prom/gaps.json`
- Create: `internal/ui/metrics.go`, `internal/views/metrics/{model,view,model_test,render_test}.go`
- Create: `internal/desktop/metrics.go`, `metrics_test.go`, `promsettings.go`
- Create icons: `memory-stick.svg`, `arrow-up-down.svg` (lucide, same LICENSE) if absent
- Modify: `internal/ui/bars.go` (extract shared plot), `internal/ui/series.go` (Cursor color), `internal/desktop/browse.go` (Pod panel), `internal/desktop/app.go` (state + reset), `internal/desktop/settings.go`, `internal/views/settings/view.go`, `internal/cluster/session.go` (discovery cache reset), `README.MD` (Metrics section)

## Implementation Steps
1. `promql.go` + table tests (exact expr strings, regex quoting, empty pod list → no queries).
2. `prometheus.go` parse + grid alignment + tests with testdata (gaps, NaN, multi-series sum, empty result).
3. Discovery + QueryRange with fake REST: use `httptest.Server` + `rest.Config{Host}` → assert path/params, 403 → `PromForbidden`, 404 everywhere → `ErrPromNotFound`, override wins.
4. Refactor `kit.Bars` → shared plot; Overview render test unchanged.
5. `kit.MetricsChart` + render test via `ui.Tester` (bars present, hover rule drawn at index, gap slot empty).
6. `views/metrics` model/view + tests (formatters, state text).
7. `desktop/metrics.go` cache/poll + tests (dedupe in-flight, stale refetch, unseen eviction, reset on context switch) with injected fetch func.
8. Wire Pod panel; Settings field + persistence test.
9. README Metrics section; `make fmt-check vet test lint`.
10. Manual: run against kube-prometheus-stack cluster, compare with Lens screenshot at 1440×900.

## Success Criteria
- [ ] Pod panel shows ~60 CPU bars for 1h immediately on kube-prometheus-stack.
- [x] Hover/←→ shows orange rule + diamond + tooltip; Esc clears.
- [x] Range 6h/24h/7d refetch, old chart stays until new data.
- [x] No KSM → no request/limit lines, no error.
- [ ] 403 → "Needs permission: services/proxy"; none found → "Prometheus not found · set in Settings".
- [x] Settings override persisted per context and used immediately.
- [x] All checks pass, no cluster needed.

## Risk Assessment
- Proxy blocked (EKS/GKE RBAC) → explicit message; port-forward is future work.
- cAdvisor label variance (`container_name` old) → unsupported, documented.
- Duplicate cAdvisor series (kubelet + cadvisor jobs) → `sum` + `image!=""` filter; verify on real cluster.
- mygo has no dashed path stroke → request/limit solid, distinguished by color + legend.
- mygo chart lacks diamond shape → approximate with small rotated-look square; acceptable.
- Poll goroutine leak → tied to session ctx; cancelled on context switch.

## Implementation Notes (2026-11-10)
- Done: promql.go, prometheus.go, ui/metrics.go (`kit.Trace`, `MetricsChart`, `MetricsLegend`), views/metrics, desktop/metrics.go + promsettings.go, Settings card, Pod panel wiring, icons, README.
- Deviations: discovery cache in desktop `App` (not Session); `kit.Bars` not refactored (Overview untouched); no link on source line; chart type named `Trace` (palette already `Series`).
- Real cluster (ght-hz, kube-prometheus-stack): CPU/network 61 pts/1h OK; KSM present; `container_fs_usage_bytes` absent on containerd → Filesystem tab promotes Reads to bars (Filesystem only).
- Code review 7.5/10 → fixed: stale fetch writes, list-denied message, promotion scoped to FS, rediscover on 404/503, context-change guard, atomic 0600 overrides write, Save/err state, block state keyed by context.
- Open: visual check vs Lens screenshot in running app (step 10).
