# PROGRESS.md

## Current State

Last commit: 45b97a7 (main); B04 branch `fix/B04-flaky-daemon-log-poll` @ 341d409 | `make check`: passing (main + bug worktree) | F01, F02, F07 passing.

- **refactor/modern-go merged via PR #69** (merge commit 3ab60a5, merge-commit style, no squash): one-commit sweep (6a5b757) of modern Go idioms — `errors.Is`, range-int loops, `sync.WaitGroup.Go` — across 69 files. Branch caught up to origin/main@35d85c7 with a clean auto-merge (zero conflicts; zcode logic + idioms both preserved), verified green locally; note the PR had **no CI runs** because the pr-check workflow only lands with PR #67 — local `make check` was the gate of record. Branch + worktree cleaned up (local + remote deleted).
- **PR #67 (pr-ci-gate) green**: hermetic `fakeRMBHome` CI fixture committed (8f1ea69) and pushed; Linux CI run passed 2m10s. Awaiting merge.
- **B04 triaged (2026-09-06)**: the transient make-check flake from the merge day recurred (3rd instance) and is now identified — `flaky: TestSpawnedDaemonStdioIsLogFdNotPipe` (issue #70, `internal/appshell`, sev-low). 5s/50ms file-poll window in `updatelatch_test.go:131-139` times out under suite load; the `*os.File` regression guard itself never fails.
- **B04 diagnosed (2026-09-06, phase 2 complete)** — branch `fix/B04-flaky-daemon-log-poll` @ 341d409, pushed. Root cause: the test's completion signal is a fixed 5s wall-clock poll for the fake daemon's `daemon-mark` write; under extreme CPU oversubscription (~30 parallel test pkgs + FTS5) the child's write lands past 5s → false FAIL at the deadline-exhaustion line (incident: 5.05s). The guarded `*os.File` assertion never failed. Causality proven by injection: test-only `RMB_TEST_DAEMON_WRITE_DELAY` hook delays the child write 8s → deterministic FAIL at ~5.0s (2/2), idle probe datapoint 410ms spawn→mark (12× headroom, why 2× load can't repro). Artifacts: `docs/bugs/B04-investigation.md`, `// B04` hook in `updatelatch_test.go` (test infra only, default off), `bug_list.json` regression {file, cmd}. Gate `make bug-state B=B04 S=diagnosed` passed (regression required to fail on unfixed code). Probe file deleted pre-commit; no product code touched; no PR yet.

## In Progress

- **B04 phase 3 (fix)**: `fix/B04-flaky-daemon-log-poll` is diagnosed, not fixed. Fix session: read `docs/bugs/B04-investigation.md`, fill a sprint contract, make the regression green (detection tolerant of a child write delayed ≥8s; keep the `*os.File` assertion untouched), then `make verify-bug B=B04` → PR.
- Issue #70 summary comment still to post (checklist item left open by the phase-2 session per mission scope).

## Next Steps

0. B04 phase 3: fix session on the existing `fix/B04-flaky-daemon-log-poll` worktree — tolerant detection, `make verify-bug B=B04`, PR (F09 pr-check), merge, close #70
1. Merge PR #67 (pr-ci-gate) so future PRs get CI runs
2. Bump VERSION, rebuild + reinstall locally so the new zcode hooks go live; remove remaining fix worktrees/panes
3. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first
4. F06 secrets → Keychain/env + key rotation (P3.5)
5. Weekly sweep per AGENTS.md Observability

## Blockers

None.

---

## Historical note (2026-08-28 session)

F07 (ZCode integration) merged at 9b70cb3, VERSION bumped to 0.2.10-dev.1 (6ef9031), local ad-hoc-signed build installed to /Applications + ~/.rmb/bin (no notarized release — SIGN_KEYCHAIN_PASS unavailable). Its flagged risk (payload shape inferred from docs) materialized as #61/#62 above.
