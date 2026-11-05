#!/bin/sh
# Release build. Stops when macOS signing credentials are absent.
# Does not commit the generated config and does not store an Apple password.
set -eu

cd "$(dirname "$0")/.."

if [ -z "${MACOS_SIGNING_IDENTITY:-}" ] || [ -z "${NOTARY_KEYCHAIN_PROFILE:-}" ]; then
	echo "stop: no signing identity"
	exit 1
fi
if [ "$MACOS_SIGNING_IDENTITY" = "-" ]; then
	echo "stop: refusing ad-hoc identity"
	exit 1
fi

base=$(mktemp)
cp mygo.json "$base"
cleanup() {
	cp "$base" mygo.json
	rm -f "$base"
}
trap cleanup EXIT

python3 - "$MACOS_SIGNING_IDENTITY" "$NOTARY_KEYCHAIN_PROFILE" "${NOTARY_KEYCHAIN:-}" <<'PY'
import json, sys
path = "mygo.json"
with open(path) as f:
    cfg = json.load(f)
identity, profile, keychain = sys.argv[1:]
cfg.setdefault("macos", {})
cfg["macos"]["signingIdentity"] = identity
cfg["macos"]["notarize"] = {"keychainProfile": profile}
if keychain:
    cfg["macos"]["notarize"]["keychain"] = keychain
cfg.setdefault("linux", {})
cfg["linux"]["webview"] = False
cfg["linux"].setdefault("maintainer", "NaKube")
cfg.pop("windows", None)
with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PY

go tool mygo build -platform darwin/universal,windows/amd64,linux/amd64
