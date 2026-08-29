# PROGRESS.md

## Current State

Last commit: 7454203 (main) | `make check`: passing | F01, F02, F07 passing.

## In Progress

**ZCode capture bugs #61 + #62** — 3-phase procedure in flight:

1. ✅ Issues filed: [#61](https://github.com/colinleefish/rmb-desktop/issues/61) (assistant-only turns), [#62](https://github.com/colinleefish/rmb-desktop/issues/62) (`sess_`-prefixed session keys)
2. ✅ Investigation complete: `docs/audit/2026-08-29-zcode-capture-bugs/INVESTIGATION.md` (root causes verified against the installed ZCode client bundle, not docs; deterministic repro; test matrix TC-1…TC-7; fixer handoff §5)
3. ✅ Fix #61 (assistant-only turns): branch `fix/zcode-61-user-prompt-capture` — two-hook design: `UserPromptSubmit → rmb hook-capture --agent=zcode` parks the prompt in `~/.rmb/cache/agent-prompts/zcode/<session>.json`; `Stop → rmb hook-submit` pairs it in `ParseZCodePayload` (sidecar consumed after read; absent ⇒ assistant-only degradation). Setup merge installs both hooks, `zcodeHookConfigured` requires both + `hooks.enabled`, stop-only installs migrate idempotently. TC-1/2/3 pass; TC-4 decision = reject camel-only payloads (INVESTIGATION.md §6). Session-key derivation untouched (that is #62).
4. ⏳ Fix #62 (`sess_`-prefixed session keys): owned by a parallel fix branch — not in this one.

## Next Steps

1. Fix #62 per INVESTIGATION.md §2 (normalize key derivation + decide migration for 3 existing rows)
2. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first
3. F06 secrets → Keychain/env + key rotation (P3.5)
4. Weekly sweep per AGENTS.md Observability

## Blockers

None. (Paused stash `WIP zcode two-hook fix` was consumed by the #61 fix branch; no longer pending.)

---

## Historical note (2026-08-28 session)

F07 (ZCode integration) merged at 9b70cb3, VERSION bumped to 0.2.10-dev.1 (6ef9031), local ad-hoc-signed build installed to /Applications + ~/.rmb/bin (no notarized release — SIGN_KEYCHAIN_PASS unavailable). Its flagged risk (payload shape inferred from docs) materialized as #61/#62 above.
