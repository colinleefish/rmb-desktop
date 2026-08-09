#!/usr/bin/env bash
# Stage Go binaries for Tauri externalBin sidecars.
# Tauri expects: src-tauri/binaries/<name>-<target-triple>[.exe]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$ROOT/app/src-tauri/binaries"
TARGET="$(rustc --print host-tuple)"
EXT=""

if [[ "$(uname -s)" == "MINGW"* || "$(uname -s)" == "MSYS"* ]]; then
  EXT=".exe"
fi

mkdir -p "$BIN_DIR"

for name in rmb rmbd; do
  src="$ROOT/bin/$name"
  dst="$BIN_DIR/${name}-${TARGET}${EXT}"
  if [[ ! -f "$src" ]]; then
    echo "prepare-sidecars: missing $src (run: make build)" >&2
    exit 1
  fi
  cp "$src" "$dst"
  chmod +x "$dst"
  echo "prepare-sidecars: $dst"
done
