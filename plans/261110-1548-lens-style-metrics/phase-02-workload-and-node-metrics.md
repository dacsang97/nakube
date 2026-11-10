---
phase: 2
title: "Workload and Node metrics"
status: pending
priority: P2
effort: "1d"
dependencies: [1]
---

# Phase 2: Workload and Node metrics

## Overview
Reuse P1 client/chart for Deployment/StatefulSet/DaemonSet (and ReplicaSet/Job) aggregates and for Nodes.

## Requirements
- Functional
  - Workload panel: same 4 tabs, aggregated over its current pods.
  - Node panel: CPU, Memory, Network, Filesystem from node-exporter; overlay = allocatable/capacity (KSM `kube_node_status_allocatable` / `_capacity`).
  - Workload with 0 pods → "No pods" state, no query.
- Non-functional: pod set recomputed each fetch (scale changes reflected next poll).

## Architecture
- Workload pods: from informer cache `a.pods`, match `spec.selector` (`metav1.LabelSelectorAsSelector`) in same namespace, filter by ownerRef chain where cheap (RS→Deploy). Pass exact names to `PodQueries` (regex `^(a|b|c)$`). Cap at 200 names; above → fallback regex `^{name}-.*` (document imprecision).
- `NodeQueries(tab, node string)` in `promql.go`:
  - instance map: `* on(instance) group_left(nodename) node_uname_info{nodename="X"}`.
  - CPU: `sum(rate(node_cpu_seconds_total{mode!="idle"}[r]) * on(instance) group_left(nodename) node_uname_info{nodename="X"})`.
  - Memory: `node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes` joined same way.
  - Network: `rate(node_network_receive_bytes_total{device!~"lo|veth.*|cali.*|cni.*|flannel.*"}[r])` rx/tx.
  - Filesystem: `node_filesystem_size_bytes - node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"}`.
- Wire: `browse.go` workload panel (non-Pod kinds), `inspect.go` `drawNodes` draw closure.

## Related Code Files
- Modify: `internal/cluster/promql.go`, `promql_test.go`, `internal/desktop/browse.go`, `internal/desktop/inspect.go`, `internal/desktop/metrics.go` (key kinds)
- Create: `internal/desktop/selector.go` + test (pods for workload)

## Implementation Steps
1. `podsFor(workload)` + tests (selector match, other ns excluded, cap fallback).
2. `NodeQueries` + tests (exact exprs, quoting).
3. Extend metricsKey kinds; wire workload + node panels.
4. Manual check vs Lens on Deployment and Node.

## Success Criteria
- [ ] Deployment panel matches Lens values ±5% on same range.
- [ ] Node panel shows data where node-exporter has `node_uname_info`; otherwise "Node metrics not available".
- [ ] Scaling a deployment changes series by next poll.

## Risk Assessment
- `node_uname_info` missing/different nodename (FQDN vs short) → also try `instance=~"X(:\\d+)?"` fallback query; fixture tests.
- Huge regex for big workloads → 200-name cap.
