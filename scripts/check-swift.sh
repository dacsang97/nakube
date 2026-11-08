#!/usr/bin/env bash
# Compile the actual iOS target; swiftc alone cannot validate its SDK/project settings.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
if [[ $# -ne 0 ]]; then
  echo "Usage: scripts/check-swift.sh" >&2
  echo "Select Xcode with DEVELOPER_DIR or xcode-select; requires Xcode 16 or newer." >&2
  exit 1
fi
if ! command -v xcodebuild >/dev/null 2>&1; then
  echo "error: install full Xcode (not only Command Line Tools) and select it with xcode-select." >&2
  exit 1
fi
xcodebuild -version
xcrun --sdk iphonesimulator --show-sdk-version

derived_data="$(mktemp -d "${TMPDIR:-/tmp}/nakube-swift.XXXXXX")"
trap 'rm -rf "$derived_data"' EXIT

for configuration in Debug Release; do
  xcodebuild \
    -project "$root/ios/NakubeRemote/NakubeRemote.xcodeproj" \
    -scheme NakubeRemote \
    -configuration "$configuration" \
    -sdk iphonesimulator \
    -destination 'generic/platform=iOS Simulator' \
    -derivedDataPath "$derived_data" \
    CODE_SIGNING_ALLOWED=NO \
    CODE_SIGNING_REQUIRED=NO \
    SWIFT_STRICT_CONCURRENCY=complete \
    SWIFT_TREAT_WARNINGS_AS_ERRORS=YES \
    build
done
