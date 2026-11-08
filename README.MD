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

Open **Settings → Na Kube Mobile** on the desktop, then use **Scan QR Code** in the phone app. Both devices must be signed in to the same Tailscale tailnet and MagicDNS must be enabled. The QR transfers only the desktop MagicDNS name and fixed port; the phone calls `http://<name>:47623` on the tailnet. Cleartext HTTP is allowed only for `ts.net`. Restart and delete confirm on the phone. A bare pod cannot be restarted.

## Release

`scripts/release.sh` stops unless `MACOS_SIGNING_IDENTITY` and `NOTARY_KEYCHAIN_PROFILE` are set. It does not commit signing fields or store an Apple password. Windows stays unsigned. Linux keeps `webview` false, so the package does not pull WebKitGTK.

## MCP

```sh
nakube mcp
```

Stdio only. It reads the context file written by desktop `Use`, or kubeconfig `current-context` when that file is absent. It can list, get, stream logs, and exec. It cannot apply, delete, or scale.
