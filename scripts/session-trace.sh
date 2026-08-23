#!/usr/bin/env bash
# session-trace.sh — structured JSONL session events (L12 observability)
#
# Usage: bash scripts/session-trace.sh <event> ['{"detail":"..."}']
#   events: session-start | session-end | verify | note
# Appends one JSON object per line to .harness/traces/traces.jsonl
# (runtime artifact — gitignored). Safe to call repeatedly; never blocks.
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
event="${1:?usage: session-trace.sh <event> [detail-json]}"
detail="${2:-{}}"
TRACE_DIR=".harness/traces"
TRACE="$TRACE_DIR/traces.jsonl"
mkdir -p "$TRACE_DIR"

agent="${PI_AGENT_ID:-${AGENT_ID:-${CLAUDECODE:+claude-code}}}"
agent="${agent:-unknown}"
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)
commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# detail must be valid JSON or it's dropped (telemetry must never break work)
echo "$detail" | jq -e . >/dev/null 2>&1 || detail='{"detail":"(unparseable)"}'

jq -cn --arg ts "$ts" --arg event "$event" --arg branch "$branch" \
  --arg commit "$commit" --arg agent "$agent" --argjson detail "$detail" \
  '{ts:$ts, event:$event, branch:$branch, commit:$commit, agent:$agent, detail:$detail}' >>"$TRACE"
echo "trace: $event @ $commit ($TRACE)"
