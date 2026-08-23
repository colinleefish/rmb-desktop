#!/usr/bin/env bash
# verify-feature.sh — harness gate (L07 WIP=1, L10 features, L11 layered verification)
#
# Runs a feature's verification layers in order (L1 static → L2 runtime → L3 system).
# On success: transitions state → passing with evidence (commit + date).
# On failure: prints the failing layer's repair instruction.
#
# Usage:
#   bash scripts/verify-feature.sh <id> [--activate]
#   make verify-feature F=<id>          # verify an active feature
#   make verify-feature F=<id> A=1      # planned → active → verify (enforces WIP=1)
#
# State machine: planned → active → passing. No skipping states.
# NEVER set state to passing by hand — that's what this script is for.
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LIST="${FEATURE_LIST:-feature_list.json}"
id="${1:-}"
flag="${2:-}"

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
green() { printf '  \033[0;32m[PASS]\033[0m %s\n' "$1"; }
red() { printf '  \033[0;31m[FAIL]\033[0m %s\n' "$1"; }

if [[ -z "$id" || "$id" == "--list" ]]; then
  bold "Features ($LIST):"
  jq -r '.features[] | "\(.state)\t\(.id)\t\(.name)\t\(.evidence)"' "$LIST" | column -t -s "$(printf '\t')"
  exit 0
fi

feature=$(jq -e --arg id "$id" '.features[] | select(.id == $id)' "$LIST" 2>/dev/null) || {
  red "feature '$id' not found in $LIST — run: make verify-feature (no args) to list"
  exit 1
}
state=$(jq -r '.state' <<<"$feature")

if [[ "$flag" == "--activate" ]]; then
  [[ "$state" == "planned" ]] || { red "cannot activate from state='$state' (must be planned)"; exit 1; }
  active_now=$(jq '[.features[] | select(.state == "active")] | length' "$LIST")
  [[ "$active_now" -eq 0 ]] || {
    active_id=$(jq -r '[.features[] | select(.state == "active")][0].id' "$LIST")
    red "WIP=1 violated: F '$active_id' is already active — complete it first"
    exit 1
  }
  jq --arg id "$id" '(.features[] | select(.id == $id)).state = "active"' "$LIST" >"$LIST.tmp" && mv "$LIST.tmp" "$LIST"
  green "$id: planned → active"
  state="active"
fi

[[ "$state" == "active" ]] || {
  red "$id state='$state' — activate first: make verify-feature F=$id A=1"
  exit 1
}

bold "Verifying $id ($(jq -r '.name' <<<"$feature"))"
layer_count=$(jq '.layers | length' <<<"$feature")
for i in $(seq 0 $((layer_count - 1))); do
  label=$(jq -r ".layers[$i].label" <<<"$feature")
  cmd=$(jq -r ".layers[$i].cmd" <<<"$feature")
  repair=$(jq -r ".layers[$i].repair" <<<"$feature")
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
  '(.features[] | select(.id == $id)).state = "passing" | (.features[] | select(.id == $id)).evidence = $ev' \
  "$LIST" >"$LIST.tmp" && mv "$LIST.tmp" "$LIST"
bold "$id: active → passing (evidence: commit $commit, $today)"
