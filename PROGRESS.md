# PROGRESS.md

## Current State

Last commit: 73dd3fa (main) — PR #67 merged, **F09 passing** (branch deleted). `make check`: passing (main; one flaky retry — B04 live, see In Progress). F01, F02, F07, F08, F09 passing. **Branch protection ON** (main): required check `check` green, PRs required (0 approvals), merge-commit only, squash/rebase disabled, `delete_branch_on_merge` on, enforce_admins **off** (docs-only commits on main stay possible).

- **F08 (bug workflow, #63) merged beefc8c**: `bug_list.json` + schema (5-state machine), `scripts/verify-bug.sh` + `make verify-bug`/`bug-state` (diagnosed gate runs the regression and requires it to FAIL; bug WIP=1 on fixing), templates `bug-report`/`bug-investigation`, `.github/ISSUE_TEMPLATE/bug.md`, clean-state D3 extended, B01–B03 backfilled (#61/#62/#15). Contract: `.harness/contracts/F08.md`.
- **F09 (pr-ci-gate, #64) merged 73dd3fa via PR #67** — the first protected merge (CI green + PR + merge commit). `pr-check.yml` = full local parity (`make setup` → `webui-build` → `check`) on ubuntu, `setup-go cache:false` (#40 race), go-build + npm caches. Its first runs caught 4 real test bugs (see below).
- **CI-caught test fixes riding #67**: #68 appshell darwin-hardcoded magic expectations (GOOS-conditional now); hermetic `fakeRMBHome` for setup preview tests (8f1ea69); #71 update manifest fixture platform-coverage + URL assertion; #72 date-rot `TestSearch_sinceUntil_filtering` hardcoded `2026-08-02` (expired 2026-09-12 — main had gone red; until now derived from fixture).
- **B05 triaged (#72)**: read-skew race in rollup materiality gate (`bucketUnchanged` reads outside the tx while a concurrent `mergeIntoIncumbent` replaces the row → false "evidence changed" → double version bump). Surfaced as `TestIncumbentMerge_CosinePath` flake on the 2nd CI run; passes locally (darwin + linux container). Root cause/evidence/fix direction on the issue; `bug_list.json` entry `reported`.

## In Progress

- **B04 phase 3 (fix)**: `fix/B04-flaky-daemon-log-poll` @ 341d409 diagnosed (5s poll window under load; `// B04` test hook + regression recorded). Fix session: make regression green (detection tolerant of ≥8s child-write delay), `make verify-bug B=B04`, PR (now gets CI), merge, close #70. Still live: 3 observed flakes incl. 2026-09-12 on main.
- **B05 → diagnosed** needs a deterministic regression (forced-interleaving hook); then phase 3 fix = move materiality read into the persist tx (or per-URI serialization).

## Next Steps

1. B04 phase-3 fix session (above); post the #70 summary comment still owed
2. B05 investigation follow-up (deterministic regression) then fix
3. WebUI refactor per `plan/webui-refactor.md`: F03 UX audit first (`make verify-feature F=F03 A=1`) — note `feature/frontend-mock-mode` branch/worktree appeared in the main checkout (2026-09-12, separate session)
4. F06 secrets → Keychain/env + key rotation (P3.5)
5. Weekly sweep per AGENTS.md Observability (audit-harness re-run, quality re-score, arch-rule promotion — candidates: no-hardcoded-dates-in-tests, no-env-dependent-test-assumptions from #68/#71)
6. Bump VERSION, rebuild + reinstall locally so the new zcode hooks go live

## Blockers

None. Notes: (a) `bug_list.json` merge conflicts across in-flight bug branches are expected/accepted (B04 branch carries its own state edits; B05 added on main via #67); (b) the shared main checkout was found on `feature/frontend-mock-mode` (another session) — this session's main-side work was done from a temp worktree `../rmb-desktop-main-tmp` to avoid disturbing it.

---

## Historical note (2026-08-28 session)

F07 (ZCode integration) merged at 9b70cb3, VERSION bumped to 0.2.10-dev.1 (6ef9031), local ad-hoc-signed build installed to /Applications + ~/.rmb/bin (no notarized release — SIGN_KEYCHAIN_PASS unavailable). Its flagged risk (payload shape inferred from docs) materialized as #61/#62 above.
