#!/usr/bin/env bash
# Assemble the RMB Desktop.app bundle (Phase 3 of plan/tauri-to-go-shell.md).
# Replaces tauri's bundler: hand-rolled Info.plist + binaries + icns + codesign.
#
# Usage: build-macos-app.sh <version> <commit> [sign-identity]
#   Expects bin/{rmb-app,rmb,rmbd} built by `make build`.
#   The in-bundle executable is named "RMB Desktop" (parity with the Tauri
#   bundle) so existing launch-at-login items keep pointing at the right file.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: build-macos-app.sh <version> <commit> [sign-identity]}"
COMMIT="${2:?usage: build-macos-app.sh <version> <commit> [sign-identity]}"
SIGN_IDENTITY="${3:-}"

APP_DIR="$ROOT/dist/RMB Desktop.app"
MACOS_DIR="$APP_DIR/Contents/MacOS"
RES_DIR="$APP_DIR/Contents/Resources"

for f in rmb-app rmb rmbd; do
  if [[ ! -f "$ROOT/bin/$f" ]]; then
    echo "build-macos-app: missing bin/$f (run: make build)" >&2
    exit 1
  fi
done
if [[ ! -f "$ROOT/icons/app.icns" ]]; then
  echo "build-macos-app: missing icons/app.icns" >&2
  exit 1
fi

echo "==> assemble $APP_DIR"
rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR" "$RES_DIR"

cp "$ROOT/bin/rmb-app" "$MACOS_DIR/RMB Desktop"
cp "$ROOT/bin/rmb"     "$MACOS_DIR/rmb"
cp "$ROOT/bin/rmbd"    "$MACOS_DIR/rmbd"
chmod +x "$MACOS_DIR/RMB Desktop" "$MACOS_DIR/rmb" "$MACOS_DIR/rmbd"

cp "$ROOT/icons/app.icns" "$RES_DIR/icon.icns"

BUILD="$(date +%Y%m%d.%H%M%S)"
sed -e "s/__VERSION__/$VERSION/" -e "s/__BUILD__/$BUILD/" \
  "$ROOT/scripts/app-template-Info.plist" > "$APP_DIR/Contents/Info.plist"

echo "==> codesign"
if [[ -n "$SIGN_IDENTITY" ]]; then
  CODESIGN_ID="$SIGN_IDENTITY"
  TIMESTAMP=(--timestamp)
else
  echo "  (no identity given — ad-hoc signing, for local use only)"
  CODESIGN_ID="-"
  TIMESTAMP=()  # secure timestamps need a real identity
fi
# Nested sidecars are separate Mach-O images: sign each one first (hardened
# runtime + secure timestamp), then seal the bundle. Otherwise notary rejects
# the DMG with "binary is not signed with a valid Developer ID certificate".
for helper in rmb rmbd; do
  codesign --force --identifier "me.remember.rmb.$helper" --options runtime \
    "${TIMESTAMP[@]}" --sign "$CODESIGN_ID" "$MACOS_DIR/$helper"
done
codesign --force --identifier "me.remember.rmb" --options runtime \
  "${TIMESTAMP[@]}" --sign "$CODESIGN_ID" "$APP_DIR"
codesign --verify --verbose=1 "$APP_DIR"

echo "build-macos-app: $APP_DIR"
plutil -p "$APP_DIR/Contents/Info.plist" | grep -E "CFBundleShortVersionString|CFBundleVersion|CFBundleExecutable"
