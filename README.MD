# NaKube

Native Kubernetes client for one kube context. The desktop window is MyGo UI, not a web page. `internal/cluster` is the only Kubernetes implementation. The window, `nakube mcp`, and the phone listener are adapters over that package.

Module: `github.com/dacsang97/nakube`  
Identifier: `io.gnas.nakube`

## Develop

Go 1.27.3 or newer.

```sh
go tool mygo dev
```

Quit the window, or press Ctrl+C, to stop. `go tool mygo dev` sets `TERM`, so the login-shell import is skipped. A Dock launch runs that import on a goroutine and does not wait for it before the first frame.

```sh
go test ./...
```

Package tests need no cluster and no display. A missing kubeconfig is not a failure.

## Phone

`ios/NakubeRemote` is a SwiftUI app. It stores a MagicDNS name and calls `http://<name>:47623` on the tailnet. `Info.plist` allows cleartext HTTP for `ts.net`. Restart and delete confirm on the phone. A bare pod cannot be restarted.

## Release

`scripts/release.sh` stops unless `MACOS_SIGNING_IDENTITY` and `NOTARY_KEYCHAIN_PROFILE` are set. It does not commit signing fields and does not store an Apple password. Windows stays unsigned. Linux keeps `webview` false.


## MCP

```sh
nakube mcp
```

Stdio only. The process reads the context file the desktop `Use` writes, or kubeconfig `current-context` when that file is absent. It can list, get, stream logs, and exec. It cannot apply, delete, or scale.

## Layout

| Path | Role |
|---|---|
| `cmd/nakube` | Desktop window and the `mcp` subcommand |
| `internal/cluster` | Kube client: watch, logs, exec, dry-run apply |
| `internal/desktop` | Native overview and sidebar |
| `internal/mcp` | Stdio MCP handler |
| `internal/remote` | Tailscale listener for the phone, port `47623` |

`linux.webview` is false. The packaged app does not pull WebKitGTK.
