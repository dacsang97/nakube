# NaKube

A native Kubernetes client for one context. The desktop window is native UI, not a web page. The phone on the same tailnet can watch and confirm a restart or delete.

`github.com/dacsang97/nakube` · `io.gnas.nakube`

## Run

Go 1.27.3 or newer.

```sh
go tool mygo dev
```

Quit the window, or press Ctrl+C.

```sh
go test ./...
```

Tests need no cluster and no display. A missing kubeconfig is not a failure.

## Editor

One tab per screen. A click opens a preview and replaces the previous one. A double click pins it. A drag reorders the strip. Logs, a shell, and YAML open beside the list. Closing the last tab returns to Overview.

| | macOS | Elsewhere |
|---|---|---|
| Close | Command-W | Ctrl-W |
| Previous / next | Command-Option-Left / Right | Ctrl-PageDown / Ctrl-PageUp |
| Jump | Command-1–8, Command-9 last | Ctrl-1–8, Ctrl-9 last |

## Phone

The phone app stores a MagicDNS name and calls `http://<name>:47623`. Restart and delete confirm on the phone. A bare pod cannot be restarted.

## MCP

```sh
nakube mcp
```

Stdio only. It can list, get, stream logs, and exec. It cannot apply, delete, or scale.
