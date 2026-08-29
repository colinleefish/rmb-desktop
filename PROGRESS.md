# PROGRESS.md

## Current State

Last commit: 7454203 (main) | `make check`: passing | F01, F02, F07 passing.

## In Progress

**ZCode capture bugs #61 + #62** — 3-phase procedure in flight:

1. ✅ Issues filed: [#61](https://github.com/colinleefish/rmb-desktop/issues/61) (assistant-only turns), [#62](https://github.com/colinleefish/rmb-desktop/issues/62) (`sess_`-prefixed session keys)
2. ✅ Investigation complete: `docs/audit/2026-08-29-zcode-capture-bugs/INVESTIGATION.md` (root causes verified against the installed ZCode client bundle, not docs; deterministic repro; test matrix TC-1…TC-7; fixer handoff §5)
3. ⏳ Fix in flight (parallel worktrees per `plan/parallel-work-and-versioning.md`):
   - **#62** — fix complete on branch `fix/zcode-62-session-key-normalize` (worktree `rmb-desktop-fix-62`): `zcodeRMBSessionID` total normalizer + tests; TC-5/TC-6 pass; `make check` green. Awaiting merge to main. TC-7 resolved as **no migration** (owner decision 2026-08-29: single-user install; the 3 legacy `sess_…` rows stay orphaned, new uploads land under fresh bare-UUID keys — see DECISIONS.md).
   - **#61** — owned by a parallel agent (message pairing / UserPromptSubmit capture); do not touch `ParseZCodePayload` pairing logic, `internal/setup/*`, `cmd/rmb/main.go` until it lands.

## Next Steps

1. Merging agent: land `fix/zcode-62-session-key-normalize` (#62), then #61's branch; both touch `internal/hook/zcode.go` — merge commits keep provenance
2. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first
3. F06 secrets → Keychain/env + key rotation (P3.5)
4. Weekly sweep per AGENTS.md Observability

## Blockers

None.

---

## Historical note (2026-08-28 session)

F07 (ZCode integration) merged at 9b70cb3, VERSION bumped to 0.2.10-dev.1 (6ef9031), local ad-hoc-signed build installed to /Applications + ~/.rmb/bin (no notarized release — SIGN_KEYCHAIN_PASS unavailable). Its flagged risk (payload shape inferred from docs) materialized as #61/#62 above.
