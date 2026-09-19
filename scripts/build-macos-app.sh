#!/usr/bin/env bash
# Assemble the RMB Desktop.app bundle (Phase 3 of plan/tauri-to-go-shell.md).
# Replaces tauri's bundler: hand-rolled Info.plist + binaries + icns + codesign.
#
# Usage: build-macos-app.sh <version> <commit> [sign-identity]
#   Expects bin/{rmb-app,rmb,rmbd} built by `make build` (override the source
#   dir with RMB_BIN_DIR, e.g. bin/intel from `make build-intel`).
#   The in-bundle executable is named "RMB Desktop" (parity with the Tauri
#   bundle) so existing launch-at-login items keep pointing at the right file.
#   Override the bundle destination with RMB_APP_DIR (default dist/RMB Desktop.app).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: build-macos-app.sh <version> <commit> [sign-identity]}"
COMMIT="${2:?usage: build-macos-app.sh <version> <commit> [sign-identity]}"
SIGN_IDENTITY="${3:-}"

BIN_DIR="${RMB_BIN_DIR:-$ROOT/bin}"
APP_DIR="${RMB_APP_DIR:-$ROOT/dist/RMB Desktop.app}"
MACOS_DIR="$APP_DIR/Contents/MacOS"
RES_DIR="$APP_DIR/Contents/Resources"

for f in rmb-app rmb rmbd; do
  if [[ ! -f "$BIN_DIR/$f" ]]; then
    echo "build-macos-app: missing $BIN_DIR/$f (run: make build / build-intel with RMB_BIN_DIR set)" >&2
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

cp "$BIN_DIR/rmb-app" "$MACOS_DIR/RMB Desktop"
cp "$BIN_DIR/rmb"     "$MACOS_DIR/rmb"
cp "$BIN_DIR/rmbd"    "$MACOS_DIR/rmbd"
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
# ${ARR[@]+...} guards: bash 3.2 (stock macOS) treats empty-array expansion
# under `set -u` as an unbound variable, which broke the ad-hoc path.
TS_ARGS=${TIMESTAMP[@]+"${TIMESTAMP[@]}"}
# Nested sidecars are separate Mach-O images: sign each one first (hardened
# runtime + secure timestamp), then seal the bundle. Otherwise notary rejects
# the DMG with "binary is not signed with a valid Developer ID certificate".
for helper in rmb rmbd; do
  codesign --force --identifier "me.remember.rmb.$helper" --options runtime \
    $TS_ARGS --sign "$CODESIGN_ID" "$MACOS_DIR/$helper"
done
codesign --force --identifier "me.remember.rmb" --options runtime \
  $TS_ARGS --sign "$CODESIGN_ID" "$APP_DIR"
codesign --verify --verbose=1 "$APP_DIR"

# Refuse to ship a binary whose Mach-O minos exceeds the deployment target.
# Building on macOS 26 without MACOSX_DEPLOYMENT_TARGET embeds minos 26.0,
# which Sequoia (15) and older cannot launch (silent kernel reject).
MAX_MINOS="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
for bin in "RMB Desktop" rmb rmbd; do
  minos="$(otool -l "$MACOS_DIR/$bin" | awk '/minos/{print $2; exit}')"
  if [[ -z "$minos" ]]; then
    echo "build-macos-app: missing LC_BUILD_VERSION.minos on $bin" >&2
    exit 1
  fi
  # Numeric compare major.minor (enough for 13.0 vs 26.0).
  awk -v a="$minos" -v b="$MAX_MINOS" 'BEGIN{
    split(a,x,"."); split(b,y,".");
    am=x[1]+0; an=(x[2]==""?0:x[2]+0);
    bm=y[1]+0; bn=(y[2]==""?0:y[2]+0);
    if (am>bm || (am==bm && an>bn)) exit 1;
    exit 0
  }' || {
    echo "build-macos-app: $bin has minos $minos > MACOSX_DEPLOYMENT_TARGET=$MAX_MINOS" >&2
    echo "  export MACOSX_DEPLOYMENT_TARGET=$MAX_MINOS before make build" >&2
    exit 1
  }
  echo "  minos ok: $bin = $minos (≤ $MAX_MINOS)"
done

echo "build-macos-app: $APP_DIR"
plutil -p "$APP_DIR/Contents/Info.plist" | grep -E "CFBundleShortVersionString|CFBundleVersion|CFBundleExecutable|LSMinimumSystemVersion"
