#!/usr/bin/env bash
# Set the custom Finder icon on a .dmg file (the file icon, not just the mounted volume).
set -euo pipefail

DMG="${1:?usage: finish-dmg.sh <file.dmg> [icon.icns]}"
ICNS="${2:-$(cd "$(dirname "$0")/.." && pwd)/icons/app.icns}"

if [[ ! -f "$DMG" ]]; then
  echo "finish-dmg: missing dmg: $DMG" >&2
  exit 1
fi
if [[ ! -f "$ICNS" ]]; then
  echo "finish-dmg: missing icon: $ICNS" >&2
  exit 1
fi

if command -v fileicon >/dev/null 2>&1; then
  fileicon set "$DMG" "$ICNS"
  echo "finish-dmg: set icon via fileicon"
  exit 0
fi

swift - "$DMG" "$ICNS" <<'SWIFT'
import AppKit
let args = CommandLine.arguments
guard args.count == 3 else { exit(1) }
let dmg = args[1]
let icns = args[2]
guard let image = NSImage(contentsOfFile: icns) else {
  fputs("finish-dmg: could not load icon\n", stderr)
  exit(1)
}
if !NSWorkspace.shared.setIcon(image, forFile: dmg, options: []) {
  fputs("finish-dmg: setIcon failed\n", stderr)
  exit(1)
}
SWIFT

echo "finish-dmg: set icon via NSWorkspace"
