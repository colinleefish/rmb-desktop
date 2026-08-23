# PROGRESS.md

## Current State

Last commit: 8073bff (merge: harness observability, #58 closed) | `make check`: passing — now includes `check-arch` as final stage | VCR: 2/2 (F01, F02 passing with evidence) | Final harness audit: 7/7 critical.

## In Progress

Nothing — **milestone harness-ready (#5) complete**: #53–#58 all closed (bonus: #59 flaky doctor test diagnosed and fixed en route).

## Next Steps

1. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first — `make verify-feature F=F03 A=1`, write `docs/audit/webui-ux-audit.md`, then F04 shell restructure, F05 settings strangler
2. F06 secrets → Keychain/env + key rotation (P3.5, last open item from `plan/memory-retrieval-remediation.md`)
3. Weekly sweep per AGENTS.md Observability: re-run `temp/learn-harness-engineering/tools/audit-harness.sh .`, re-score `docs/quality-document.md`, promote review findings into `.harness/arch-rules.json`

## Blockers

None.
