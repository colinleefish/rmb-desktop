#!/usr/bin/env bash
# verify-bug.sh — bug workflow gate (F08), mirror of verify-feature.sh (L07/L10/L11)
#
# Three-phase bug procedure (AGENTS.md "Bug workflow"), one phase = one agent session:
#   phase 1 triage:         issue filed + entry added            → reported
#   phase 2 investigation:  claim                                → investigating
#                           doc + failing regression test        → diagnosed  (gate runs the
#                                                                  test, REQUIRES it to fail)
#   phase 3 fix:            branch claimed, bug WIP=1            → fixing
#                           layers green (this script)           → passing
#
# Usage:
#   bash scripts/verify-bug.sh --list
#   bash scripts/verify-bug.sh <id> --state <next>   # gated transition (adjacent only)
#   bash scripts/verify-bug.sh <id>                  # final verify: fixing → passing
#
# State machine: reported → investigating → diagnosed → fixing → passing.
# No skipping states. NEVER set state to passing by hand — that's what this script is for.
# Run inside the bug's worktree (gates execute against the caller's checkout).
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LIST="${BUG_LIST:-bug_list.json}"

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
green() { printf '  \033[0;32m[PASS]\033[0m %s\n' "$1"; }
red() { printf '  \033[0;31m[FAIL]\033[0m %s\n' "$1"; }

trace() { # best-effort observability (L12); telemetry must never block work
  bash scripts/session-trace.sh bug-state "$1" >/dev/null 2>&1 || true
}

set_state() { # set_state <id> <state>
  jq --arg id "$1" --arg st "$2" \
    '(.bugs[] | select(.id == $id)).state = $st' "$LIST" >"$LIST.tmp" && mv "$LIST.tmp" "$LIST"
}

adjacent() { # adjacent <from> <to>
  case "$1:$2" in
    reported:investigating | investigating:diagnosed | diagnosed:fixing) return 0 ;;
    *) return 1 ;;
  esac
}

if [[ -z "${1:-}" || "$1" == "--list" ]]; then
  bold "Bugs ($LIST):"
  jq -r '.bugs[] | "\(.state)\t\(.id)\t#\(.issue)\t\(.title)\t\(.evidence // "")"' "$LIST" |
    column -t -s "$(printf '\t')"
  exit 0
fi

id="$1"
mode="${2:-}"
bug=$(jq -e --arg id "$id" '.bugs[] | select(.id == $id)' "$LIST" 2>/dev/null) || {
  red "bug '$id' not found in $LIST — run: make verify-bug (no args) to list"
  exit 1
}
state=$(jq -r '.state' <<<"$bug")

# ── Gated transitions ────────────────────────────────────────────────────────
if [[ "$mode" == "--state" ]]; then
  target="${3:?usage: verify-bug.sh <id> --state <investigating|diagnosed|fixing>}"
  if [[ "$target" == "passing" ]]; then
    red "passing is set only by the final verify: make verify-bug B=$id"
    exit 1
  fi
  adjacent "$state" "$target" || {
    red "cannot advance $state → $target (adjacent transitions only, no skipping)"
    exit 1
  }

  case "$target" in
    investigating)
      # claim — free transition; parallel investigations are allowed
      ;;
    diagnosed)
      # phase-2 exit gate: the investigation must actually reproduce the bug
      doc=$(jq -r '.investigation // empty' <<<"$bug")
      if [[ -z "$doc" || ! -f "$doc" ]]; then
        red "investigation doc missing or not found: '$doc' (expected docs/bugs/$id-investigation.md)"
        exit 1
      fi
      grep -qi '^## .*root cause' "$doc" || {
        red "investigation doc has no '## Root cause' section — see templates/bug-investigation.md (backfilled docs: add the H2 on the bug branch)"
        exit 1
      }
      rfile=$(jq -r '.regression.file // empty' <<<"$bug")
      rcmd=$(jq -r '.regression.cmd // empty' <<<"$bug")
      if [[ -z "$rfile" || -z "$rcmd" ]]; then
        red "bug entry lacks regression {file, cmd} — record the failing test in $LIST"
        exit 1
      fi
      [[ -f "$rfile" ]] || { red "regression file not found: $rfile"; exit 1; }
      grep -q "$id" "$rfile" || {
        red "regression file does not mention $id — tag the test with a '// $id' comment"
        exit 1
      }
      if out=$(bash -c "$rcmd" 2>&1); then
        red "regression PASSED on unfixed code — the bug was not reproduced"
        exit 1
      fi
      if ! grep -q 'FAIL' <<<"$out"; then
        red "regression cmd failed without a test FAIL (compile error?) — output tail:"
        tail -5 <<<"$out" | sed 's/^/      /'
        exit 1
      fi
      green "regression reproduces: test fails on unfixed code"
      ;;
    fixing)
      branch=$(jq -r '.branch // empty' <<<"$bug")
      [[ -n "$branch" ]] || { red "bug entry lacks branch — set fix/$id-<slug>"; exit 1; }
      if ! git rev-parse --verify --quiet "refs/heads/$branch" >/dev/null &&
        ! git rev-parse --verify --quiet "refs/remotes/origin/$branch" >/dev/null; then
        red "branch '$branch' not found locally or on origin"
        exit 1
      fi
      wip=$(jq '[.bugs[] | select(.state == "fixing")] | length' "$LIST")
      if [[ "$wip" -ne 0 ]]; then
        wip_id=$(jq -r '[.bugs[] | select(.state == "fixing")][0].id' "$LIST")
        red "bug WIP=1 violated: '$wip_id' is already fixing — complete it first"
        exit 1
      fi
      ;;
  esac

  set_state "$id" "$target"
  trace "{\"bug\":\"$id\",\"from\":\"$state\",\"to\":\"$target\"}"
  green "$id: $state → $target"
  exit 0
fi

# ── Final verify: fixing → passing (runs the entry's layers) ─────────────────
if [[ "$mode" != "" ]]; then
  red "unknown mode '$mode' — usage: verify-bug.sh <id> [--state <next>]"
  exit 1
fi
[[ "$state" == "fixing" ]] || {
  red "$id state='$state' — advance to fixing first: make bug-state B=$id S=fixing"
  exit 1
}

bold "Verifying $id ($(jq -r '.title' <<<"$bug"))"
layer_count=$(jq '.layers | length' <<<"$bug")
for i in $(seq 0 $((layer_count - 1))); do
  label=$(jq -r ".layers[$i].label" <<<"$bug")
  cmd=$(jq -r ".layers[$i].cmd" <<<"$bug")
  repair=$(jq -r ".layers[$i].repair" <<<"$bug")
  if bash -c "$cmd" >/dev/null 2>&1; then
    green "$label"
  else
    red "$label — $cmd"
    echo "    repair: $repair"
    exit 1
  fi
done

commit=$(git rev-parse --short HEAD 2>/dev/null || echo uncommitted)
today=$(date -u +%Y-%m-%d)
jq --arg id "$id" --arg ev "commit $commit, verified $today" \
  '(.bugs[] | select(.id == $id)).state = "passing" |
   (.bugs[] | select(.id == $id)).evidence = $ev' \
  "$LIST" >"$LIST.tmp" && mv "$LIST.tmp" "$LIST"
trace "{\"bug\":\"$id\",\"from\":\"fixing\",\"to\":\"passing\"}"
bold "$id: fixing → passing (evidence: commit $commit, $today)"
echo "merge → close issue #$(jq -r '.issue' <<<"$bug") → append 'issue closed' to evidence"
