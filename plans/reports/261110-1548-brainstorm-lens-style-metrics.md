---
type: brainstorm
date: 2026-11-10
topic: Lens-style metrics in resource detail panels
status: approved
---

# Brainstorm: Lens-style metrics panel

## Summary
Add a "Metrics" block to detail panels that behaves like Lens: Prometheus-backed history, 4 tabs, range dropdown, hover marker, request/limit overlay, source line. Ship in 3 phases.

## Problem / requirements
- Ref: Lens pod panel — range `1h`, tabs CPU/Mem/Network/FS, bars, orange hover rule + diamond, "Displaying metrics from Prometheus: ns / svc:port".
- Resources: Pod, Workloads (Deploy/STS/DS), Node, Container-in-pod.
- Tabs: CPU, Memory, Network, Filesystem.
- Must-haves: range 1h/6h/24h/7d; hover rule+marker+tooltip; source line; request/limit overlay.
- Source: Prometheus first, fallback metrics-server.
- Discovery: auto + per-context override in Settings (`ns/svc:port`).
- Overlay: kube-state-metrics (like Lens). No KSM → overlay hidden, never invented.
- Refresh: poll 60s while panel visible + on range/tab change; keep old data until new arrives.

## Codebase context (scout)
- Go native desktop, `egoist/mygo` v0.7.4. `mygo/ui/chart` has `Plot`, `Rect`, `Line`, `RuleX`, `Point`, `Select` (hover + arrows), `Linear.Nice/Ticks`.
- `internal/ui/bars.go` `kit.Bars` — time bars w/ tooltip, used by Overview.
- `internal/cluster/metrics.go` — metrics-server nodes only; no `PodMetricses`; no Prometheus code.
- History today = in-RAM `a.samples` (`internal/desktop/app.go`), trimmed 25h.
- Shared detail panel: `offerDetailPanel` / `shellDetailPanel` (`internal/desktop/inspect.go`), workloads `Detail` (`internal/views/workloads/view.go`).
- Series palette: `internal/ui/series.go`.

## Approaches evaluated
| | Transport | Pros | Cons |
|---|---|---|---|
| **A (chosen)** | API-server service proxy `/api/v1/namespaces/{ns}/services/{svc}:{port}/proxy/api/v1/query_range` via client-go `RESTClient` | Lens's method; no tunnel; kubeconfig auth | needs RBAC `services/proxy`; some managed clusters block |
| B | port-forward to Prometheus pod | works when proxy blocked | tunnel lifecycle, reconnect, complexity |
| C | external URL (Thanos/Grafana Cloud) | flexible | auth/TLS; not Lens-like; YAGNI |

403 → explicit message ("needs services/proxy"), no silent fallback.

## Final design
1. **`internal/cluster/prometheus.go`**
   - `DiscoverPrometheus(ctx)`: override → `monitoring/prometheus-operated:9090` → `*/kube-prometheus-stack-prometheus` → `*/prometheus-server` → `lens-metrics/prometheus`. Cache per context.
   - `QueryRange(ctx, src, promql, start, end, step) ([]Series, error)`; missing points = gap (OK false), not 0.
   - Step = range/60 (~60 bars): 1h→60s, 6h→6m, 24h→24m, 7d→168m.
2. **`internal/cluster/promql.go`** — pure builders (`PodQueries(ns, pods, container)`, `WorkloadQueries`, `NodeQueries`), Lens "operator" flavor (cAdvisor + KSM + node-exporter). Modern labels only (`container`, not `container_name`).
3. **`internal/ui/metrics.go`** — shared chart, extends `kit.Bars`:
   - primary series = bars; secondary (requests/limits/transmit/reads/writes) = lines.
   - right Y axis ticks, X time ticks.
   - hover via `chart.Select` → orange `RuleX` + diamond `Point` + tooltip (time + all series). ←/→ keys.
   - toolbar: range dropdown, 4 icon toggles (cpu, memory, arrow-up-down, hard-drive); source line; legend below.
   - states: loading skeleton, discovering, metrics-server fallback, "Metrics not available".
4. **Tabs / series**
   | Tab | Series |
   |---|---|
   | CPU | usage (bar), requests, limits |
   | Memory | working set (bar), requests, limits |
   | Network | receive (bar), transmit |
   | Filesystem | usage (bar), reads/s, writes/s |
5. **State** in `App` keyed `(context, kind, ns/name, container, range)`; fetch in goroutine → invalidate; poll 60s only while visible.

## Phases
- **P1**: prometheus.go + promql.go + shared chart + Pod panel (4 tabs, hover, range, overlay, source line) + Settings override.
- **P2**: Workloads (exact pod-name regex from selector via informer cache) + Node (node-exporter, join `kube_node_info` for node→instance).
- **P3**: container dropdown + metrics-server fallback (`PodMetricses`, in-RAM sampling, CPU/Mem only, "since HH:MM").

## Risks
- Node→instance mapping (P2) — Lens's weakest spot; fixture tests.
- Label variance across stacks — support modern labels, document.
- 7d sparse bars — accepted (Lens same).
- Managed clusters denying `services/proxy`.

## Acceptance (P1)
- Pod on kube-prometheus-stack cluster: 1h history shows immediately.
- Hover → orange rule + values; range 6h reloads.
- No KSM → bars only, no request/limit lines.
- No Prometheus → clear message.
- Unit tests: PromQL builders, response parsing (testdata), discovery order. `make test` needs no cluster.

## Out of scope
External URL / Thanos / Grafana, auth, provider-type dropdown, alerting, export.

## Next steps
`/ght:plan` with this report → phase files P1–P3.

## Unresolved questions
- Exact Settings storage location for per-context override (reuse existing settings view persistence — confirm during planning).
