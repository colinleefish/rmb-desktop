# PROGRESS.md

## Current State

Last commit: 43fa3b7 (docs: plan lifecycle cleanup) | `make check`: passing — verified 2026-08-24 on `chore/harness-foundation` (2 consecutive green runs; recall@5=0.500 ≥ 0.50 gate, dup-rate=0, recency-precision=1.0). Note: one non-reproducible flaky test failure on the very first run under load.

## In Progress

- **harness-ready refactor** (milestone [#5](https://github.com/colinleefish/rmb-desktop/milestone/5)):
  - #53 AGENTS.md + tool scoping — **done, closing**
  - #54 PROGRESS.md + DECISIONS.md — **done, closing**
  - #55 make check/setup/dev + runtime pins — **done, closing**
  - #56 feature_list.json + verify-feature gate — next
  - #57 arch-rules + check-arch
  - #58 observability + clean-state protocol

## Next Steps

1. #56 feature_list.json + `scripts/verify-feature.sh` + `make vcr` (branch `chore/harness-features`)
2. #57 `.harness/arch-rules.json` + `scripts/check-arch.sh`, wire into `make check`
3. #58 templates (sprint-contract, evaluator-rubric, clean-state-checklist) + session-trace + `docs/quality-document.md`
4. P3.5 from `plan/memory-retrieval-remediation.md`: secrets → Keychain/env + key rotation (last open item)
5. WebUI refactor per `plan/webui-refactor.md` (starts week of 2026-08-31; first = UX audit → `docs/audit/webui-ux-audit.md`)

## Blockers

None.
