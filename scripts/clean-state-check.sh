#!/usr/bin/env bash
# clean-state-check.sh — idempotent verifier of the 5 clean-state dimensions (L12)
# Corresponds to templates/clean-state-checklist.md. Exit 0 only when all pass.
# Usage: bash scripts/clean-state-check.sh [repo-path]
set -uo pipefail

REPO="${1:-.}"
cd "$REPO" || exit 1
GO_TAGS="sqlite_fts5"
pass=0; fail=0
ok()  { printf '  \033[0;32m[PASS]\033[0m %s\n' "$1"; pass=$((pass+1)); }
bad() { printf '  \033[0;31m[FAIL]\033[0m %s\n' "$1"; fail=$((fail+1)); }

# D1 build passes
if CGO_ENABLED=1 go build -tags "$GO_TAGS" ./... >/dev/null 2>&1; then
  ok "D1 build passes"
else
  bad "D1 build passes — run: go build -tags $GO_TAGS ./..."
fi

# D2 tests pass
if CGO_ENABLED=1 go test -tags "$GO_TAGS" ./... >/dev/null 2>&1; then
  ok "D2 tests pass"
else
  bad "D2 tests pass — run: go test -tags $GO_TAGS ./..."
fi

# D3 feature list valid & consistent (WIP<=1, passing ⇒ evidence)
if command -v jq >/dev/null 2>&1 && jq -e '
    ([.features[] | select(.state=="active")] | length) <= 1
    and all(.features[] | select(.state=="passing"); (.evidence // "") != "")
    and all(.features[]; .state == "planned" or .state == "active" or .state == "passing")
  ' feature_list.json >/dev/null 2>&1; then
  ok "D3 feature list updated"
else
  bad "D3 feature list updated — jq check failed on feature_list.json (states valid? ≤1 active? passing has evidence?)"
fi

# D4 no debug artifacts
if git ls-files | grep -qE '\.(orig|rej|swp)$|(^|/)\.DS_Store$'; then
  bad "D4 no debug artifacts — tracked .orig/.rej/.swp/.DS_Store present (git rm)"
else
  ok "D4 no debug artifacts"
fi

# D5 startup path works (embedded webui present; no stale binaries claim)
if [[ -f internal/http/static/web/index.html ]]; then
  ok "D5 startup path works"
else
  bad "D5 startup path works — missing embedded webui, run: make webui-build"
fi

echo "clean-state: $pass/5"
[[ "$fail" -eq 0 ]] || exit 1
