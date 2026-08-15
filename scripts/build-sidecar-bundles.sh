#!/usr/bin/env bash
# Build the sidecar update bundles + signed manifest for a release
# (Phase 2 of plan/tauri-to-go-shell.md). These are what the self-updater
# downloads — the .app shell is NOT in the bundles (it updates rarely).
#
# Usage: build-sidecar-bundles.sh <version> <commit>
#   Expects bin/{rmb,rmbd} built (`make build`) and, for Windows,
#   bin/{rmb,rmbd}-windows-amd64.exe (`make build-windows-sidecars`).
#   Emits dist/rmb-desktop_<ver>_darwin_arm64.tar.gz,
#   dist/rmb-desktop_<ver>_windows_amd64.zip, dist/manifest.json (signed).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: build-sidecar-bundles.sh <version> <commit>}"
COMMIT="${2:?usage: build-sidecar-bundles.sh <version> <commit>}"

MAC_BUNDLE="rmb-desktop_${VERSION}_darwin_arm64.tar.gz"
WIN_BUNDLE="rmb-desktop_${VERSION}_windows_amd64.zip"
RELEASED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

mkdir -p "$ROOT/dist"

echo "==> macOS sidecar bundle ($MAC_BUNDLE)"
if [[ ! -f "$ROOT/bin/rmb" || ! -f "$ROOT/bin/rmbd" ]]; then
  echo "build-sidecar-bundles: missing bin/rmb or bin/rmbd (run: make build)" >&2
  exit 1
fi
tar czf "$ROOT/dist/$MAC_BUNDLE" -C "$ROOT/bin" rmb rmbd
MAC_SHA="$(shasum -a 256 "$ROOT/dist/$MAC_BUNDLE" | cut -d' ' -f1)"

echo "==> Windows sidecar bundle ($WIN_BUNDLE)"
if [[ -f "$ROOT/bin/rmb-windows-amd64.exe" && -f "$ROOT/bin/rmbd-windows-amd64.exe" ]]; then
  STAGE="$(mktemp -d)"
  trap 'rm -rf "$STAGE"' EXIT
  cp "$ROOT/bin/rmb-windows-amd64.exe"  "$STAGE/rmb.exe"
  cp "$ROOT/bin/rmbd-windows-amd64.exe" "$STAGE/rmbd.exe"
  (cd "$STAGE" && zip -r -q "$ROOT/dist/$WIN_BUNDLE" rmb.exe rmbd.exe)
  WIN_SHA="$(shasum -a 256 "$ROOT/dist/$WIN_BUNDLE" | cut -d' ' -f1)"
  WIN_JSON="$(jq -nc --arg f "$WIN_BUNDLE" --arg s "$WIN_SHA" '{sidecars:$f, sha256:$s}')"
else
  echo "  (skip: bin/*-windows-amd64.exe missing — run make build-windows-sidecars)"
  WIN_JSON='null'
fi

echo "==> manifest"
jq -n \
  --arg ver "$VERSION" \
  --arg released "$RELEASED_AT" \
  --arg macfile "$MAC_BUNDLE" \
  --arg macsha "$MAC_SHA" \
  --argjson win "$WIN_JSON" \
  '{product:"rmb-desktop", version:$ver, released_at:$released,
    platforms:{macos:{aarch64:{sidecars:$macfile, sha256:$macsha}},
               windows:{amd64:$win}}}' \
  > "$ROOT/dist/manifest.json"

# Sign if the release key is present; the updater refuses unsigned manifests,
# so an unsigned output is a hard signal that the key is missing.
if [[ -n "${RMB_MANIFEST_KEY:-}" || -f "$HOME/.rmb/release-keys/manifest.key" ]]; then
  echo "==> sign manifest"
  go run "$ROOT/cmd/manifest-sign" sign "$ROOT/dist/manifest.json"
else
  echo "build-sidecar-bundles: WARNING manifest is NOT signed (key missing at ~/.rmb/release-keys/manifest.key)" >&2
fi

echo "build-sidecar-bundles:"
echo "  $ROOT/dist/$MAC_BUNDLE"
[[ "$WIN_JSON" != "null" ]] && echo "  $ROOT/dist/$WIN_BUNDLE"
echo "  $ROOT/dist/manifest.json"
