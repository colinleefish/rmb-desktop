#!/usr/bin/env bash
# Build the Windows x64 distribution zip (Phase 3, decision D1: zip not NSIS).
#
# Usage: build-windows-zip.sh <version> <commit>
#   Expects bin/{rmb,rmbd}-windows-amd64.exe from build-windows-sidecars.sh.
#   Cross-compiles the pure-Go tray shell rmb-app.exe here.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: build-windows-zip.sh <version> <commit>}"
COMMIT="${2:?usage: build-windows-zip.sh <version> <commit>}"
ARCHIVE_NAME="RMB-Desktop_${VERSION}_x64"
STAGE="$ROOT/dist/$ARCHIVE_NAME"
ZIP="$ROOT/dist/$ARCHIVE_NAME.zip"

GO_LDFLAGS="-X github.com/colinleefish/rmb-desktop/internal/version.Version=${VERSION} -X github.com/colinleefish/rmb-desktop/internal/version.Commit=${COMMIT}"

for f in rmb rmbd; do
  if [[ ! -f "$ROOT/bin/$f-windows-amd64.exe" ]]; then
    echo "build-windows-zip: missing bin/$f-windows-amd64.exe (run: make build-windows-sidecars)" >&2
    exit 1
  fi
done

echo "==> cross-compile rmb-app.exe"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -C "$ROOT" -ldflags "$GO_LDFLAGS" -o "$ROOT/bin/rmb-app-windows-amd64.exe" ./cmd/rmb-app

echo "==> stage $STAGE"
rm -rf "$STAGE"
mkdir -p "$STAGE"
cp "$ROOT/bin/rmb-app-windows-amd64.exe" "$STAGE/rmb-app.exe"
cp "$ROOT/bin/rmb-windows-amd64.exe"    "$STAGE/rmb.exe"
cp "$ROOT/bin/rmbd-windows-amd64.exe"   "$STAGE/rmbd.exe"
cp "$ROOT/scripts/install-windows.ps1"  "$STAGE/install.ps1"

rm -f "$ZIP"
(cd "$ROOT/dist" && zip -r -q "$ARCHIVE_NAME.zip" "$ARCHIVE_NAME")
rm -rf "$STAGE"

echo "build-windows-zip: $ZIP"
unzip -l "$ZIP"
