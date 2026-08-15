#!/usr/bin/env bash
# Create the release DMG from an assembled .app (Phase 3 of
# plan/tauri-to-go-shell.md). Replaces tauri's dmg bundler.
#
# Usage: build-dmg.sh <version>   (expects dist/RMB Desktop.app to exist)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: build-dmg.sh <version>}"
APP="$ROOT/dist/RMB Desktop.app"
DMG="$ROOT/dist/RMB Desktop_${VERSION}_aarch64.dmg"

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
