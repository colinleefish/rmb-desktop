#!/usr/bin/env bash
# Submit agent hook payloads to both local rmb-desktop and remote rmb server.
#
# Binary layout (~/.rmb/bin/):
#   rmb-desktop  — local-first CLI (posts to rmbd on 127.0.0.1:19019)
#   rmb          — CS client CLI (reads ~/.rmb/config.yaml → RMB_URL)
#
# Install CS client: ./install.sh from github.com/colinleefish/rmb
set -euo pipefail

SOURCE="${1:?usage: rmb-hook-dual <cursor|cc>}"
PAYLOAD="$(cat)"

DESKTOP_BIN="${RMB_DESKTOP_BIN:-$HOME/.rmb/bin/rmb-desktop}"
REMOTE_BIN="${RMB_REMOTE_BIN:-$HOME/.rmb/bin/rmb}"
LOCAL_URL="${RMB_LOCAL_URL:-http://127.0.0.1:19019}"

is_desktop_cli() {
  local bin="$1"
  [[ -x "$bin" ]] && strings "$bin" 2>/dev/null | grep -q 'colinleefish/rmb-desktop'
}

rc=0
if ! printf '%s' "$PAYLOAD" | "$DESKTOP_BIN" hook-submit --source="$SOURCE" --url="$LOCAL_URL"; then
  rc=1
fi

if is_desktop_cli "$REMOTE_BIN"; then
  echo "rmb-hook-dual: $REMOTE_BIN is rmb-desktop (local), not the CS client." >&2
  echo "  Reinstall server client: curl -fsSL https://raw.githubusercontent.com/colinleefish/rmb/main/install.sh | bash" >&2
  echo "  Or: go build -o ~/.rmb/bin/rmb ./cmd/rmb  (from rmb repo)" >&2
  exit 1
fi

if ! printf '%s' "$PAYLOAD" | "$REMOTE_BIN" hook-submit --source="$SOURCE"; then
  rc=1
fi
exit "$rc"
