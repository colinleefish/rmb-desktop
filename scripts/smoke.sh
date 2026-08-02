#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DB_DIR="$(mktemp -d)"
DB_PATH="${DB_DIR}/rmb.db"
export RMB_DB_PATH="${DB_PATH}"
export RMB_ADDR="127.0.0.1:19019"

cleanup() {
  if [[ -n "${PID:-}" ]]; then
    kill "${PID}" 2>/dev/null || true
    wait "${PID}" 2>/dev/null || true
  fi
  rm -rf "${DB_DIR}"
}
trap cleanup EXIT

cd "${ROOT}"
go run -tags sqlite_fts5 ./cmd/rmbd serve &
PID=$!
sleep 1

curl -fsS "http://${RMB_ADDR}/healthz" | grep -q '"status"'

PAYLOAD='{"conversation_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","cursor_version":"1.0","status":"completed","text":"smoke test reply"}'
echo "${PAYLOAD}" | go run -tags sqlite_fts5 ./cmd/rmb hook-submit --source=cursor --url="http://${RMB_ADDR}"

echo "smoke ok"
