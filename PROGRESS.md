# PROGRESS.md

## Current State

Last commit: 541bd25 (main) | `make check`: passing | F01, F02, F07 passing.

- **refactor/modern-go merged via PR #69** (merge commit 3ab60a5, merge-commit style, no squash): one-commit sweep (6a5b757) of modern Go idioms — `errors.Is`, range-int loops, `sync.WaitGroup.Go` — across 69 files. Branch caught up to origin/main@35d85c7 with a clean auto-merge (zero conflicts; zcode logic + idioms both preserved), verified green locally; note the PR had **no CI runs** because the pr-check workflow only lands with PR #67 — local `make check` was the gate of record. Branch + worktree cleaned up (local + remote deleted).
- **PR #67 (pr-ci-gate) green**: hermetic `fakeRMBHome` CI fixture committed (8f1ea69) and pushed; Linux CI run passed 2m10s. Awaiting merge.
- **B04 triaged (2026-09-06)**: the transient make-check flake from the merge day recurred (3rd instance) and is now identified — `flaky: TestSpawnedDaemonStdioIsLogFdNotPipe` (issue #70, `internal/appshell`, sev-low). 5s/50ms file-poll window in `updatelatch_test.go:131-139` times out under suite load; the `*os.File` regression guard itself never fails. Needs a phase-2 investigation session.

## In Progress

Nothing in flight (zcode #61/#62 both merged via PRs #65/#66; modern-go merged via #69).

## Next Steps

0. B04 phase 2: investigation session on `fix/B04-<slug>` — deterministic repro under load + regression tag (`// B04`), then fix the poll window
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
