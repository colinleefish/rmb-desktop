# PROGRESS.md

## Current State

Last commit: 7454203 (main) | `make check`: passing | F01, F02, F07 passing.

## In Progress

**ZCode capture bugs #61 + #62** — 3-phase procedure in flight:

1. ✅ Issues filed: [#61](https://github.com/colinleefish/rmb-desktop/issues/61) (assistant-only turns), [#62](https://github.com/colinleefish/rmb-desktop/issues/62) (`sess_`-prefixed session keys)
2. ✅ Investigation complete: `docs/audit/2026-08-29-zcode-capture-bugs/INVESTIGATION.md` (root causes verified against the installed ZCode client bundle, not docs; deterministic repro; test matrix TC-1…TC-7; fixer handoff §5)
3. ⏳ Fix: NOT started — handed off to the fixing agent. A paused direct-fix attempt is stashed on branch `fix/zcode-user-prompt-capture` (see INVESTIGATION.md §5.4).

## Next Steps

1. Fixing agent: implement per INVESTIGATION.md (acceptance = TC-1…TC-7), branch + worktree per `plan/parallel-work-and-versioning.md`
2. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first
3. F06 secrets → Keychain/env + key rotation (P3.5)
4. Weekly sweep per AGENTS.md Observability

## Blockers

None.

---

## Historical note (2026-08-28 session)

F07 (ZCode integration) merged at 9b70cb3, VERSION bumped to 0.2.10-dev.1 (6ef9031), local ad-hoc-signed build installed to /Applications + ~/.rmb/bin (no notarized release — SIGN_KEYCHAIN_PASS unavailable). Its flagged risk (payload shape inferred from docs) materialized as #61/#62 above.
