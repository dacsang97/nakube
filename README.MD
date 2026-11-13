# NaKube

A native desktop app for one Kubernetes context.

The window is native, so it stays fast. The interface is quiet: clear type, calm color, and room to work.

`github.com/dacsang97/nakube` · `io.gnas.nakube`

## Develop

Go 1.27.3 or newer.

```sh
go tool mygo dev
```

Quit the window, or press Ctrl+C. `go tool mygo dev` sets `TERM`, so the login-shell import is skipped. A Dock launch runs that import in the background and does not wait for the first frame.

```sh
go test ./...
```

Tests need no cluster and no display. A missing kubeconfig is not a failure.

## Editor

One tab per screen. A click opens a preview and replaces the previous one. A double click pins it. A drag reorders the strip. Logs, a shell, and YAML open beside the list. The context rail switches clusters without leaving the current screen; expand it to show context names, with provider details added when short names collide. Namespace-scoped list tabs keep their own namespace filter; their Command/Ctrl-P palette can switch to one namespace or back to All namespaces. Workloads combines live health summaries, readiness charts, per-kind status rails, search, filters, and a sortable inventory. Nodes, Namespaces, Events, generic Kubernetes resources, and Argo resource lists open selected-row details and actions in the shared right shell panel. Closing the last tab returns to Overview.

| | macOS | Elsewhere |
|---|---|---|
| Toggle context rail | Command-Shift-E | Ctrl-Shift-E |
| Close | Command-W | Ctrl-W |
| Previous / next | Command-Option-Left / Right | Ctrl-PageDown / Ctrl-PageUp |
| Jump | Command-1–8, Command-9 last | Ctrl-1–8, Ctrl-9 last |
| Namespace palette | Command-P | Ctrl-P |

## Settings

**Catalog** always loads the built-in kubeconfig directory and can sync additional kubeconfig files or folders. **Paste kubeconfig** validates YAML before storing it as a private `0600` file; synced sources can be removed without deleting the original file or folder.

## Pods

The Pods list shows these columns: Name, Namespace, Containers, CPU, Memory, Restarts, Controlled By, Node, QoS, Age, and Status. Each column but Containers sorts. CPU and memory come from metrics-server every 15 seconds while the list is shown, and read N/A when it has no sample. Status reads as kubectl's. Controlled By and Node open their Workloads or Nodes row in a pinned tab; an owner of another kind, as a custom resource, is plain text.

Owner and Node links support Tab focus and Enter or Space activation, with a visible focus outline and at least a 40×40 desktop hit area. Container indicators wrap within their column rather than hiding additional containers. Loading uses the same table columns and horizontal scrolling as the inventory.

## Metrics

A pod's details open on a Metrics block: CPU, memory, network, and filesystem over 1h, 6h, 24h, or 7d, about 60 bars per range. Point at a bar, or use the arrow keys, for its values.

The metric tabs support arrow keys, Home, and End, and wrap below the range selector in narrow panels. Filesystem usage (bytes) and read/write throughput (bytes/s) use separate charts and axes, so capacity and rates are never compared as the same unit.

Data comes from Prometheus through the API server's service proxy, so the account needs `services/proxy`. The app looks for `prometheus-operated`, `kube-prometheus-stack-prometheus`, `prometheus-kube-prometheus-prometheus`, `prometheus-server`, and `prometheus`, with the `monitoring` namespace first. **Settings → Metrics** can name another service, as `namespace/service:port`, for each context. Usage is cAdvisor with its `container` label. Request and limit lines come from kube-state-metrics, and are left out when it is not installed. The chart fetches again every minute while it is shown.

## Checks

CI runs these on pull requests and on `main`. They do not package the app.

```sh
make fmt-check
make vet
make test
make lint
```

## Phone

Open **Settings → Na Kube Mobile** on the desktop. A phone that has never connected opens directly into the QR scanner; scan the desktop code to connect. Both devices must be signed in to the same Tailscale tailnet and MagicDNS must be enabled. The QR transfers only the desktop MagicDNS name and fixed port; the phone calls `http://<name>:47623` on the tailnet. Cleartext HTTP is allowed only for `ts.net`.

After a successful connection, the phone opens the context catalog on subsequent launches. Choose a context to open the Cluster and Workloads areas. Cluster contains Overview, Nodes, Namespaces, Events, Leases, and process-owned Port-forwards. Workloads contains the aggregate inventory plus Pods, Deployments, ReplicaSets, DaemonSets, StatefulSets, Jobs, and CronJobs. Every list is searchable, opens a native detail screen, and supports pull to refresh. Pods additionally expose live container logs, confirmed restart for controller-owned pods, and confirmed delete. Reading or mutating the chosen context on the phone does not switch the desktop context or overwrite its persisted selection. Missing metrics-server is shown as unavailable, not zero. Port-forwards are read-only on iPhone and remain owned by the desktop process.

The phone catalog uses `GET /v1/contexts` (credential-free name, cluster, and current fields). Context-scoped Cluster screens use `/v1/cluster/{overview|nodes|namespaces|events|leases}?context=<name>`. Workload inventory uses `GET /v1/workloads[/{resource}]?context=<name>`; pod logs, restart, and delete are nested below `/v1/workloads/pods/{namespace}/{name}` and carry the same context query. `GET /v1/port-forwards` returns the desktop process's active forwards. Legacy top-level `/v1/overview` and `/v1/pods` routes are not served.

The overview's numeric `charts` payload drives native CPU/memory utilization bars and node/pod health charts. Usage is compared with allocatable capacity; percentages above 100% remain visible. An absent or partial node-metrics response is unavailable, not zero. Loading shows layout-matched skeletons that respect Reduce Motion; no synthetic history is plotted.

To run the iOS app on an iPhone, open `ios/NakubeRemote/NakubeRemote.xcodeproj`, select the `NakubeRemote` target, and keep **Signing & Capabilities → Automatically manage signing** enabled. Select your Apple development team and the connected iPhone, then Run. Debug and Release use automatic signing; do not disable `CODE_SIGNING_ALLOWED` or `CODE_SIGNING_REQUIRED` for device builds.

### iOS source layout

The iOS app uses a responsibility-based layout under `ios/NakubeRemote/NakubeRemote`:

- `ContentView.swift` composes onboarding or contexts home; it does not own networking or persistence.
- `Core/AppConnectionStore.swift` owns the verified desktop connection, legacy-address migration, and navigation reset identity. `Core/DesktopAddress.swift` validates QR/manual MagicDNS addresses.
- `Models/` contains API response values. `Services/` owns HTTP requests, decoding, context-identity validation, and connection error messages. `DesktopAPI` accepts a `URLSession` for isolated verification.
- `Views/Connection/`, `Views/Contexts/`, `Views/Cluster/`, `Views/Overview/`, and `Views/Workloads/` own feature UI and view-scoped tasks. Shared UI lives in `Views/Components/`; capture lifecycle and the serial camera queue are isolated in `Views/Connection/Scanner/`.

Xcode 16+ automatically includes Swift files through a filesystem-synchronized app group. Add files inside the app source directory without hand-editing build-file entries. `Info.plist` is explicitly excluded from automatic resource membership.

## Release

`scripts/release.sh` stops unless `MACOS_SIGNING_IDENTITY` and `NOTARY_KEYCHAIN_PROFILE` are set. It does not commit signing fields or store an Apple password. Windows stays unsigned. Linux keeps `webview` false, so the package does not pull WebKitGTK.

For iOS version/tag releases, Swift checks, and the Xcode Cloud tag-trigger setup, see [`ios/RELEASE.md`](ios/RELEASE.md). iOS tags use `ios-v…`, separate from desktop releases.

## MCP

```sh
nakube mcp
```

Stdio only. It reads the context file written by desktop `Use`, or kubeconfig `current-context` when that file is absent. It can list, get, stream logs, and exec. It cannot apply, delete, or scale.
