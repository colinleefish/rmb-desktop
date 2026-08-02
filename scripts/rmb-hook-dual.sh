#!/usr/bin/env bash
# Submit agent hook payloads to both local rmb-desktop and remote rmb server.
set -euo pipefail

SOURCE="${1:?usage: rmb-hook-dual <cursor|cc>}"
PAYLOAD="$(cat)"

DESKTOP_BIN="${RMB_DESKTOP_BIN:-$HOME/.rmb/bin/rmb-desktop}"
REMOTE_BIN="${RMB_REMOTE_BIN:-$HOME/.rmb/bin/rmb}"
LOCAL_URL="${RMB_LOCAL_URL:-http://127.0.0.1:19019}"

rc=0
if ! printf '%s' "$PAYLOAD" | "$DESKTOP_BIN" hook-submit --source="$SOURCE" --url="$LOCAL_URL"; then
  rc=1
fi
if ! printf '%s' "$PAYLOAD" | "$REMOTE_BIN" hook-submit --source="$SOURCE"; then
  rc=1
fi
exit "$rc"
