---
title: Shared resource detail panels
date: 2026-11-06
summary: Unified selected-resource details and primary actions in one shell-owned right panel across the app.
---

# Shared resource detail panels

## What happened
Workload details used the shell-level right card while Nodes, Namespaces, Events, generic Kubernetes lists, and Argo lists rendered details or actions inside their main content.

## Decision
Generalized the right shell card into one reusable detail panel with per-tab open state. Migrated all row-detail resource screens to supply a title, copy value, supported header actions, and body. The shared panel owns width, border, radius, close behavior, scrolling, header height, neutral header color, and shell alignment. Its 35px header matches the sidebar and main tab strip and intentionally has no bottom divider. Copy sits beside the resource name; edit, remove, and close use a compact icon toolbar. After copying, the copy icon becomes a check for two seconds before resetting. YAML/edit and delete/uninstall actions live in the header while contextual actions such as logs, exec, sync, rollback, scale, restart, and forwarding remain in the body.

## Verification
- Added selection, close, per-tab state, shell geometry, header alignment, action separation, clipboard, two-second copy-feedback transition, and action-dispatch coverage.
- Rendered and inspected the Workloads panel at 1440×900 with the title-adjacent copy action and edit, remove, and close toolbar.
- Verified specialized resources, generic Kubernetes resources, workloads, Helm action mapping, and Argo resource action mapping.
- `make fmt-check`, `make vet`, and `make test` pass with the race detector.

## Next steps
None.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
