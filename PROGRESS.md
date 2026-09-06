# PROGRESS.md

## Current State

Last commit: 7454203 (main) | `make check`: passing | F01, F02, F07 passing.

## In Progress

**ZCode capture bugs #61 + #62** — 3-phase procedure in flight:

1. ✅ Issues filed: [#61](https://github.com/colinleefish/rmb-desktop/issues/61) (assistant-only turns), [#62](https://github.com/colinleefish/rmb-desktop/issues/62) (`sess_`-prefixed session keys)
2. ✅ Investigation complete: `docs/audit/2026-08-29-zcode-capture-bugs/INVESTIGATION.md` (root causes verified against the installed ZCode client bundle, not docs; deterministic repro; test matrix TC-1…TC-7; fixer handoff §5)
3. ✅ Both fixes complete, integrated on the stacked branch `fix/zcode-62-session-key-normalize` (#65 already merged to main; this PR carries the remaining #62 changes):
   - **#61 (merged, PR #65)** — two-hook capture: `UserPromptSubmit → rmb hook-capture --agent=zcode` parks the prompt in `~/.rmb/cache/agent-prompts/zcode/<session>.json`; `Stop → rmb hook-submit` pairs it in `ParseZCodePayload` (sidecar consumed after read; absent ⇒ assistant-only). Setup installs both hooks, `zcodeHookConfigured` requires both + `hooks.enabled`, stop-only installs migrate idempotently. Sidecar lookup deliberately decoupled from the stored session key (regression-tested).
   - **#62 (this PR)** — `zcodeRMBSessionID` total normalizer: bare UUIDs pass through, `sess_<uuid>` stripped, everything else (subagent ids, garbage) → deterministic uuid5. TC-7 resolved as **no migration** (owner decision 2026-08-29: single-user install; 3 legacy `sess_…` rows stay orphaned — see DECISIONS.md).
   - Composition verified on this branch: pairing works across key normalization (regression tests), `make check` green.

## Next Steps

1. Integrator: merge PR #66, then bump VERSION, rebuild + reinstall locally so the new hooks go live; remove fix worktrees/panes
2. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first
3. F06 secrets → Keychain/env + key rotation (P3.5)
4. Weekly sweep per AGENTS.md Observability

## Blockers

None. (Paused stash `WIP zcode two-hook fix` was consumed by the #61 fix branch; no longer pending.)

---

## Historical note (2026-08-28 session)

F07 (ZCode integration) merged at 9b70cb3, VERSION bumped to 0.2.10-dev.1 (6ef9031), local ad-hoc-signed build installed to /Applications + ~/.rmb/bin (no notarized release — SIGN_KEYCHAIN_PASS unavailable). Its flagged risk (payload shape inferred from docs) materialized as #61/#62 above.
