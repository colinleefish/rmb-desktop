#!/usr/bin/env bash
# Create the release DMG from an assembled .app (Phase 3 of
# plan/tauri-to-go-shell.md). Replaces tauri's dmg bundler.
#
# Usage: build-dmg.sh <version> [arch=aarch64]   (expects dist/RMB Desktop.app
#   to exist; override the source bundle with RMB_APP_DIR, e.g. the Intel
#   bundle at dist/intel/RMB Desktop.app from `make app-build-intel`)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: build-dmg.sh <version> [arch=aarch64]}"
ARCH="${2:-aarch64}"
APP="${RMB_APP_DIR:-$ROOT/dist/RMB Desktop.app}"
DMG="$ROOT/dist/RMB Desktop_${VERSION}_${ARCH}.dmg"

if [[ ! -d "$APP" ]]; then
  echo "build-dmg: missing $APP (run: scripts/build-macos-app.sh)" >&2
  exit 1
fi

STAGING="$(mktemp -d)/dmg"
trap 'rm -rf "$(dirname "$STAGING")"' EXIT
mkdir -p "$STAGING"
cp -R "$APP" "$STAGING/"
ln -s /Applications "$STAGING/Applications"

echo "==> create $DMG"
rm -f "$DMG"
hdiutil create \
  -volname "RMB Desktop" \
  -srcfolder "$STAGING" \
  -format UDZO \
  -ov \
  "$DMG" >/dev/null

# Custom Finder icon on the dmg file itself.
bash "$ROOT/scripts/finish-dmg.sh" "$DMG" "$ROOT/icons/app.icns"

echo "build-dmg: $DMG"
