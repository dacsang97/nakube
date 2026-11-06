# NaKube

Native Kubernetes client for one context.

The window is native UI. `internal/cluster` is the only Kubernetes implementation. The window, `nakube mcp`, and the phone are adapters over it.

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

One tab per screen.

A click opens a preview and replaces the previous one. A double click pins it. A drag reorders the strip. Logs, a shell, and YAML open beside the list. The last close returns to Overview.

| | macOS | Elsewhere |
|---|---|---|
| Close | Command-W | Ctrl-W |
| Previous / next | Command-Option-Left / Right | Ctrl-PageDown / Ctrl-PageUp |
| Jump | Command-1–8, Command-9 last | Ctrl-1–8, Ctrl-9 last |

The tab menu closes the current tab, the others, the tabs to the right, or all of them.

## Checks

CI runs these on pull requests and on `main`. They do not package the app.

```sh
make fmt-check
make vet
make test
make lint
```

## Phone

`ios/NakubeRemote` stores a MagicDNS name and calls `http://<name>:47623` on the tailnet. Cleartext HTTP is allowed for `ts.net`. Restart and delete confirm on the phone. A bare pod cannot be restarted.

## Release

`scripts/release.sh` stops unless `MACOS_SIGNING_IDENTITY` and `NOTARY_KEYCHAIN_PROFILE` are set. It does not commit signing fields or store an Apple password. Windows stays unsigned. Linux keeps `webview` false, so the package does not pull WebKitGTK.

## MCP

```sh
nakube mcp
```

Stdio only. It reads the context file written by desktop `Use`, or kubeconfig `current-context` when that file is absent. It can list, get, stream logs, and exec. It cannot apply, delete, or scale.

## Layout

| Path | Role |
|---|---|
| `cmd/nakube` | Window and `mcp` |
| `internal/cluster` | Watch, logs, exec, dry-run apply |
| `internal/ui` | Widgets. No Kubernetes types |
| `internal/views` | One package per screen |
| `internal/desktop` | Sidebar, tabs, router, session |
| `internal/mcp` | Stdio MCP |
| `internal/remote` | Phone listener on port `47623` |
