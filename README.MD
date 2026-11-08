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

After a successful connection, the phone opens the context catalog on subsequent launches. Choose a context to view its cluster overview: node readiness, pods, warnings, CPU, and memory, with the same summaries as desktop. Pull to refresh. Reading a context on the phone does not switch the desktop context or overwrite its persisted selection. Missing metrics-server is shown as unavailable, not zero. The mobile interface currently exposes overview only; existing desktop pod and Argo APIs are unchanged.

The phone catalog uses `GET /v1/contexts` (credential-free name, cluster, and current fields); overview uses `GET /v1/overview?context=<name>`. Requests without a context retain the desktop-session overview behavior.

The overview's numeric `charts` payload drives native CPU/memory utilization bars and node/pod health charts. Usage is compared with allocatable capacity; percentages above 100% remain visible. An absent or partial node-metrics response is unavailable, not zero. Loading shows layout-matched skeletons that respect Reduce Motion; no synthetic history is plotted.

To run the iOS app on an iPhone, open `ios/NakubeRemote/NakubeRemote.xcodeproj`, select the `NakubeRemote` target, and keep **Signing & Capabilities → Automatically manage signing** enabled. Select your Apple development team and the connected iPhone, then Run. Debug and Release use automatic signing; do not disable `CODE_SIGNING_ALLOWED` or `CODE_SIGNING_REQUIRED` for device builds.

### iOS source layout

The iOS app uses a responsibility-based layout under `ios/NakubeRemote/NakubeRemote`:

- `ContentView.swift` composes onboarding or contexts home; it does not own networking or persistence.
- `Core/AppConnectionStore.swift` owns the verified desktop connection, legacy-address migration, and navigation reset identity. `Core/DesktopAddress.swift` validates QR/manual MagicDNS addresses.
- `Models/` contains API response values. `Services/` owns HTTP requests, decoding, context-identity validation, and connection error messages. `DesktopAPI` accepts a `URLSession` for isolated verification.
- `Views/Connection/`, `Views/Contexts/`, and `Views/Overview/` own feature UI and view-scoped tasks. Shared UI lives in `Views/Components/`; capture lifecycle and the serial camera queue are isolated in `Views/Connection/Scanner/`.

Xcode 16+ automatically includes Swift files through a filesystem-synchronized app group. Add files inside the app source directory without hand-editing build-file entries. `Info.plist` is explicitly excluded from automatic resource membership.

## Release

`scripts/release.sh` stops unless `MACOS_SIGNING_IDENTITY` and `NOTARY_KEYCHAIN_PROFILE` are set. It does not commit signing fields or store an Apple password. Windows stays unsigned. Linux keeps `webview` false, so the package does not pull WebKitGTK.

For iOS version/tag releases, Swift checks, and the Xcode Cloud tag-trigger setup, see [`ios/RELEASE.md`](ios/RELEASE.md). iOS tags use `ios/v…`, separate from desktop releases.

## MCP

```sh
nakube mcp
```

Stdio only. It reads the context file written by desktop `Use`, or kubeconfig `current-context` when that file is absent. It can list, get, stream logs, and exec. It cannot apply, delete, or scale.
