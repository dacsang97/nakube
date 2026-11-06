---
title: Workloads overview and detail panel
date: 2026-11-06
summary: Rebuilt Workloads as a live health dashboard with reusable status modeling and a polished details panel.
---

# Workloads overview and detail panel

## What happened
The Workloads screen only summarized pod phases and did not expose the watched workload controllers already available in the cluster session.

## Decision
Added a reusable workload normalization model for Pods, Deployments, ReplicaSets, DaemonSets, StatefulSets, Jobs, and CronJobs. The native screen now presents semantic health totals, an overall readiness chart, per-kind status rails, search and health filters, and a sortable inventory. Selecting a row opens a dedicated right-side shell panel with the same border, radius, background, height, and gutter treatment as the navigation and main-content cards. Its header inherits the outer card surface instead of painting across the rounded border, keeping both top corners consistent with the other shell panels. The panel contains a fixed title bar, status summary, overview, capacity, and close action; implicit double-click navigation and decorative shadow were removed.

## Verification
- Rendered and inspected responsive dashboard frames, the complete three-panel desktop shell at 1440px, and the right-panel corner borders after inset alignment.
- Added model, loading, responsive render, watched-resource integration, explicit row-selection panel state, close interaction, and shell geometry coverage.
- `make fmt-check`, `make vet`, and `make test` pass.
- The native development app built and launched under `go tool mygo dev`.

## Next steps
Install `golangci-lint` on the workstation before running the optional `make lint` target.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
