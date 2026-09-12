# PROGRESS.md

## Current State

Last commit: 14fe897 (feature/frontend-mock-mode) — F10 merged origin/main (a9210da) into the branch; **F10 passing** (make verify-feature F=F10, L1/L2/L3 green, 2026-09-12). `make check`: passing on this branch. F01, F02, F07, F08, F09, **F10** passing. Branch protection ON (main): required check `check`, PRs required, merge-commit only, enforce_admins off.

- **F10 (webui-dev-mock) complete ce5bef9 + merge 14fe897** (contract `.harness/contracts/F10.md`): `cd webui && npm run dev:mock` runs the whole SPA on an in-memory deterministic dataset (~36 sessions w/ turns/atoms/scenes, 60 memories, 12 skills w/ file trees, corrections, config, derived overview/pipeline-health) with **no daemon** — central flag `mockMode.ts` (VITE_MOCK=true or localStorage `rmb.mock`), legacy per-feature mocks honor it, mutations work in memory (config PUT, corrections, onboarding, restart phases), node selftest (49 asserts) pins the router contract (L2). Corner badge shows MOCK DATA / LIVE DATA and toggles modes without restarting the server. `npm run dev` proxies to the real daemon via `RMB_API_TARGET` (default 127.0.0.1:19019 — NOT 19090). Dev loop documented in `webui/README.md`; webui quality row re-graded D→C tests, C→B docs.
- **F10 branch not yet pushed/PR'd** — owner asked for work on this branch; push + PR (now CI-gated) is the natural next step.
- **Process miss recorded**: this session started from a stale local main (af6b33b) — never fetched at clock-in. Two consequences, both cleaned up: (a) re-triaged the already-fixed #71 date bomb as duplicate issue #73 + PR #74 (both closed, branch/worktree deleted; corrected duplicate pointer → #71); (b) initially blocked on red `make check` (date bomb + B04 flake) before the merge brought main's fix in. Lesson candidate for the weekly sweep: clock-in should include `git fetch` + check origin/main before baseline.
- **F08 (bug workflow, #63) merged beefc8c**: `bug_list.json` + schema (5-state machine), `scripts/verify-bug.sh` + `make verify-bug`/`bug-state` (diagnosed gate runs the regression and requires it to FAIL; bug WIP=1 on fixing), templates `bug-report`/`bug-investigation`, `.github/ISSUE_TEMPLATE/bug.md`, clean-state D3 extended, B01–B03 backfilled (#61/#62/#15). Contract: `.harness/contracts/F08.md`.
- **F09 (pr-ci-gate, #64) merged 73dd3fa via PR #67** — the first protected merge (CI green + PR + merge commit). `pr-check.yml` = full local parity (`make setup` → `webui-build` → `check`) on ubuntu, `setup-go cache:false` (#40 race), go-build + npm caches. Its first runs caught 4 real test bugs (see below).
- **CI-caught test fixes riding #67**: #68 appshell darwin-hardcoded magic expectations (GOOS-conditional now); hermetic `fakeRMBHome` for setup preview tests (8f1ea69); #71 update manifest fixture platform-coverage + URL assertion; #72 date-rot `TestSearch_sinceUntil_filtering` hardcoded `2026-08-02` (expired 2026-09-12 — main had gone red; until now derived from fixture).
- **B05 triaged (#72)**: read-skew race in rollup materiality gate (`bucketUnchanged` reads outside the tx while a concurrent `mergeIntoIncumbent` replaces the row → false "evidence changed" → double version bump). Surfaced as `TestIncumbentMerge_CosinePath` flake on the 2nd CI run; passes locally (darwin + linux container). Root cause/evidence/fix direction on the issue; `bug_list.json` entry `reported`.

## In Progress

- **B04 phase 3 (fix)**: `fix/B04-flaky-daemon-log-poll` @ 341d409 diagnosed (5s poll window under load; `// B04` test hook + regression recorded). Fix session: make regression green (detection tolerant of ≥8s child-write delay), `make verify-bug B=B04`, PR (now gets CI), merge, close #70. Still live: 3 observed flakes incl. 2026-09-12 on main (flaked again in the F10 session's first full check; passed solo + on later full runs).
- **B05 → diagnosed** needs a deterministic regression (forced-interleaving hook); then phase 3 fix = move materiality read into the persist tx (or per-URI serialization).

## Next Steps

0. Push `feature/frontend-mock-mode` + open PR (CI now runs) → merge to main; F10 evidence points at 14fe897
1. B04 phase-3 fix session (above); post the #70 summary comment still owed
2. B05 investigation follow-up (deterministic regression) then fix
3. WebUI refactor per `plan/webui-refactor.md`: F03 UX audit first (`make verify-feature F=F03 A=1`) — F10's mock mode is the dev loop for that work
4. F06 secrets → Keychain/env + key rotation (P3.5)
5. Weekly sweep per AGENTS.md Observability (audit-harness re-run, quality re-score, arch-rule promotion — candidates: no-hardcoded-dates-in-tests, no-env-dependent-test-assumptions from #68/#71, clock-in-must-fetch from the F10 session)
6. Bump VERSION, rebuild + reinstall locally so the new zcode hooks go live

## Blockers

None. Notes: (a) `bug_list.json` merge conflicts across in-flight bug branches are expected/accepted (B04 branch carries its own state edits; B05 added on main via #67); (b) untracked in the main checkout, predating the F10 session and left alone: `.agents/`, `.claude/skills/`, `exports/`, `icons/logo.png`, `skills-lock.json`.

---

## Historical note (2026-08-28 session)

F07 (ZCode integration) merged at 9b70cb3, VERSION bumped to 0.2.10-dev.1 (6ef9031), local ad-hoc-signed build installed to /Applications + ~/.rmb/bin (no notarized release — SIGN_KEYCHAIN_PASS unavailable). Its flagged risk (payload shape inferred from docs) materialized as #61/#62 above.
