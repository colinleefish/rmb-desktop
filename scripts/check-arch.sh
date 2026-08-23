#!/usr/bin/env bash
# check-arch.sh — architectural boundary rule runner (L09/L10)
#
# Rules live in .harness/arch-rules.json: {id, description, check, expect,
# what, why, fix}. `check` is a shell command; `expect` is an anchored regex
# matched against its collapsed stdout. Violations print WHAT/WHY/FIX.
#
# Promotion principle: every new error category caught in code review becomes
# a rule here — rules are how the repo remembers its scars.
set -uo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
RULES="${RULES:-.harness/arch-rules.json}"

[[ -f "$RULES" ]] || { echo "check-arch: missing $RULES"; exit 1; }

pass=0; fail=0
while IFS= read -r rule; do
  id=$(jq -r '.id' <<<"$rule")
  desc=$(jq -r '.description' <<<"$rule")
  cmd=$(jq -r '.check' <<<"$rule")
  expect=$(jq -r '.expect' <<<"$rule")
  what=$(jq -r '.what' <<<"$rule")
  why=$(jq -r '.why' <<<"$rule")
  fix=$(jq -r '.fix' <<<"$rule")

  out=$(bash -c "$cmd" 2>/dev/null | tr '\n' ' ' | sed 's/^ *//;s/ *$//')
  if [[ -n "$expect" && "$out" =~ $expect ]]; then
    printf '  \033[0;32m[PASS]\033[0m %s — %s\n' "$id" "$desc"
    pass=$((pass + 1))
  else
    printf '  \033[0;31m[FAIL]\033[0m %s — %s\n' "$id" "$desc"
    echo "    WHAT: $what (check output: '$out', expected: '$expect')"
    echo "    WHY:  $why"
    echo "    FIX:  $fix"
    fail=$((fail + 1))
  fi
done < <(jq -c '.rules[]' "$RULES")

echo "arch rules: $pass pass, $fail fail"
[[ "$fail" -eq 0 ]] || exit 1
