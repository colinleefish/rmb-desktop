#!/usr/bin/env bash
# Cross-compile Go sidecars for Windows x86_64.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${TARGET:-x86_64-pc-windows-msvc}"
VERSION="${VERSION:-0.1.12}"
COMMIT="${COMMIT:-$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
GO_LDFLAGS="-X github.com/colinleefish/rmb-desktop/internal/version.Version=${VERSION} -X github.com/colinleefish/rmb-desktop/internal/version.Commit=${COMMIT}"

SQLITE_INC="$(go list -m -f '{{.Dir}}' github.com/mattn/go-sqlite3)"
TMPINC="$(mktemp -d)"
trap 'rm -rf "$TMPINC"' EXIT
cp "$SQLITE_INC/sqlite3-binding.h" "$TMPINC/sqlite3.h"
cp "$SQLITE_INC/sqlite3ext.h" "$TMPINC/"

export PATH="/opt/homebrew/opt/mingw-w64/bin:${PATH:-}"
export CGO_ENABLED=1
export GOOS=windows
export GOARCH=amd64
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++
export CGO_CFLAGS="-I$TMPINC"

mkdir -p "$ROOT/bin"

for name in rmb rmbd; do
  out="$ROOT/bin/${name}-windows-amd64.exe"
  go build -C "$ROOT" -tags sqlite_fts5 -ldflags "$GO_LDFLAGS" -o "$out" "./cmd/${name}"
  echo "build-windows-sidecars: $out"
done
