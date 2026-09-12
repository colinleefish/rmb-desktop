# Bug Investigation — B04: flaky TestSpawnedDaemonStdioIsLogFdNotPipe 5s log poll times out under full-suite load

> **Phase 2 artifact** (AGENTS.md "Bug workflow"). This doc is the fix agent's ONLY context —
> write it so they never re-investigate. Commit as `docs/bugs/B04-investigation.md` on the
> bug's `fix/B04-flaky-daemon-log-poll` branch (never directly on main). When done:
> 1. Record `regression = {file, cmd}` on the bug entry in `bug_list.json` ✅ (this branch)
> 2. Post a one-paragraph summary comment on the issue
> 3. `make bug-state B=B04 S=diagnosed` — the gate runs the regression and **requires it to
>    FAIL on unfixed code**. A passing test means the bug was not reproduced.

- **Bug ID / Issue**: B04 / #70
- **Investigator**: pi coding agent session (phase 2; continues a context-exhausted predecessor session that had proven the mechanism but written nothing)
- **Date**: 2026-09-06
- **Branch**: `fix/B04-flaky-daemon-log-poll` (base main@45b97a7)

## TL;DR — root cause in one line

`internal/appshell/updatelatch_test.go:152-160` (`TestSpawnedDaemonStdioIsLogFdNotPipe`)
gates on a **fixed 5-second wall-clock poll** (50 ms tick) for the fake daemon's
`daemon-mark` write — under extreme CPU oversubscription the child's write becomes visible
after 5 s, so the test false-FAILs even though the guarded behavior (direct `*os.File` log
fd) is correct.

## Evidence chain

Every step: observation → command → key output → conclusion (`file:line`).

1. **Incident (issue #70)**: `make check` failed with
   `--- FAIL: TestSpawnedDaemonStdioIsLogFdNotPipe (5.05s)` at `updatelatch_test.go:138`
   (pre-#69 numbering), "child write via direct fd never reached the daemon log", green on
   immediate re-run, always green isolated. Test duration = 5.05 s = the poll window
   exhausted, not a hang — the test polled the full 5 s and gave up.
   → Failure mode is *detection-window exhaustion*, not a broken assertion.

2. **What the test actually guards** — the `*os.File` type assertion
   (`updatelatch_test.go:127-135` post-#69 numbering) **never failed** in any run,
   including the incident. Only the write-visibility poll failed.
   → The flake lives in the *poll*, not in the guarded behavior.

3. **Poll mechanism**: `updatelatch_test.go:152-160` — `deadline := 5 * time.Second`,
   read `platform.DaemonLogPath()` file, `strings.Contains(..., "daemon-mark")`,
   `time.Sleep(50ms)`. The completion signal is the child's `echo daemon-mark` (written by
   the fake daemon script, `updatelatch_test.go:124`) landing in a file the parent reads.
   Visibility latency = child scheduling (exec, sh startup, write, page-cache) + parent
   poll tick. Under load, *both* sides are delayed; the 50 ms tick also drifts (each
   `os.ReadFile` + sleep iteration costs > 50 ms when the runloop is descheduled).

4. **Load statistics (predecessor session, 2026-09-06)**: 224 loaded runs of this test
   across trees at bases `fee7787`, `d14cadd`+modernization, `main` post-#69 — **0
   failures**. The machine (M5 Pro) never crossed the 5 s margin at the achievable ~2×
   suite concurrency; the incident run had far worse oversubscription (~30 parallel test
   packages + FTS5 recall-eval queries saturating cores).
   → The flake needs *extreme* oversubscription; ordinary local runs cannot repro it on
   demand.

5. **Fresh idle datapoint (this session, probe `TestB04ProbeSpawnToMark`, 60 s window —
   temporary instrument, run once, then deleted as planned)**:
   ```
   RESULT start->Start()ret=1.993584ms start->mark=410.406167ms polls=8 maxPollGap=51.26475ms
   --- PASS: TestB04ProbeSpawnToMark (0.41s)
   ```
   → Idle spawn→mark latency is **410 ms**: ~12× headroom under the 5 s window; poll tick
   drift negligible when unloaded. The incident therefore required >12× scheduling
   degradation of a `sh` one-liner — consistent with a saturated machine, impossible to hit
   at 2× load.

6. **Causality proof — deterministic delay injection** (test-only env hook
   `RMB_TEST_DAEMON_WRITE_DELAY`, `updatelatch_test.go:24`, `// B04`): delaying the fake
   daemon's first write by 8 s replays the incident in slow motion:
   ```
   $ RMB_TEST_DAEMON_WRITE_DELAY=8s CGO_ENABLED=1 go test -tags sqlite_fts5 -count=1 \
       ./internal/appshell -run TestSpawnedDaemonStdioIsLogFdNotPipe
   --- FAIL: TestSpawnedDaemonStdioIsLogFdNotPipe (5.01s)
       updatelatch_test.go:160: child write via direct fd never reached the daemon log
   FAIL    github.com/colinleefish/rmb-desktop/internal/appshell   5.232s
   ```
   2/2 runs, FAIL at 5.01–5.02 s (vs incident 5.05 s), exit 1. Without the delay the same
   test passes in 0.67 s (also re-verified this session).
   → A child write delayed past 5 s **is sufficient** to cause the exact observed failure;
   the poll window is the defect, the `*os.File` fd semantics are fine.

**Flaky-bug statistics summary**: incident run 1 fail / ~3 full-suite invocations on a
saturated day; 224 loaded runs at ≤2× concurrency: 0 fails; deterministic repro via 8 s
injected delay: 2/2 fails at ~5.0 s; failing interval is exactly the fixed 5 s window.

## Root cause

- **Defect location**: `internal/appshell/updatelatch_test.go:152-160` — the completion
  signal of `TestSpawnedDaemonStdioIsLogFdNotPipe` is a **fixed 5 s wall-clock poll for
  the child's `daemon-mark` write**, with a 50 ms tick that itself drifts under load.
- **What is wrong**: the test treats "child write visible within 5 s" as the pass
  condition for "child holds the log fd directly". The 5 s constant is a load-blind
  guess: on an oversubscribed machine (the incident: ~30 parallel test packages + FTS5
  queries), the fake daemon's `exec`/`sh` startup and its write can be scheduled later
  than 5 s, and the polling goroutine's 50 ms tick stretches as well. The test then
  reports the guarded behavior as broken when only the *observation deadline* was
  exceeded.
- **Why it produces the symptom**: the incident's `--- FAIL ... (5.05s)` at
  `updatelatch_test.go:138` (old numbering; `:160` today) is precisely the
  deadline-exhaustion path `t.Error("child write via direct fd never reached the daemon
  log")`. Because the write eventually lands (milliseconds later, next time the test is
  re-run), the failure is non-deterministic, load-sensitive, and always green in
  isolation — exactly the reported flake. Causal chain closed by step 6: inject an 8 s
  write delay → same failure line, same 5 s exhaustion, 2/2 runs.

## Regression test(s)

Committed in this phase; tagged with `// B04`. Expected to FAIL before the fix, PASS
after. Recorded in `bug_list.json` as `regression = {file, cmd}`.

| Test | File | Expected (post-fix) | Actual (pre-fix, observed) |
|---|---|---|---|
| `TestSpawnedDaemonStdioIsLogFdNotPipe` run with `RMB_TEST_DAEMON_WRITE_DELAY=8s` | `internal/appshell/updatelatch_test.go` | PASS — detection tolerates a child write delayed ≥8 s | FAIL at ~5.01 s: "child write via direct fd never reached the daemon log" (2/2 runs) |

Mechanism: `b04DelayedMarkScript` (`updatelatch_test.go:24`, test infrastructure only —
no product code) reads `RMB_TEST_DAEMON_WRITE_DELAY` (`time.ParseDuration`, unset/`0s` =
no delay) and prepends `sleep <d>` to the fake daemon script so its first write lands
after the current 5 s window. The regression is the *same* guarded test run under that
delay — a slow-motion, deterministic replay of the incident (write at 8 s vs window 5 s).

Run: `RMB_TEST_DAEMON_WRITE_DELAY=8s CGO_ENABLED=1 go test -tags sqlite_fts5 -count=1 ./internal/appshell -run TestSpawnedDaemonStdioIsLogFdNotPipe`

## Fix direction (not a patch)

- **Suggested approach**: make the *detection* tolerant of slow children while keeping the
  guarded behavior identical. Two acceptable shapes (fix agent chooses):
  1. **Event/sentinel-based readiness**: block on something that fires when the child
     actually writes, instead of a wall-clock window — e.g. have the fake daemon `touch` a
     sentinel file after the mark and wait on it with a generous, delay-derived timeout
     (`5s + 2×RMB_TEST_DAEMON_WRITE_DELAY` at minimum; the delay hook must be able to
     extend whatever window the test uses), or poll with an unbounded/long deadline inside
     a test-only context.
  2. **Load-adequate window**: derive the deadline from the injected delay
     (`window = base + 2×delay`) so the regression (8 s delay) exercises the *slow child*
     path without ever tripping on the deadline — while the unfixed 5 s constant, which is
     what phase 3 replaces, keeps failing it.
  The regression passes iff the test observes a child write that arrives after 5 s. Any
  fix that merely re-runs the test faster or skips under delay is wrong.
- **Files to touch**: `internal/appshell/updatelatch_test.go` only (test infrastructure).
  **No product code** (`internal/appshell/daemon.go` fd handling is correct and guarded).
- **Risks / edge cases**:
  - MUST keep the `*os.File` type assertion (`updatelatch_test.go:127-135`) **untouched**
    — that is the actual regression guard for the v0.2.5 SIGPIPE incident.
  - The delay hook must remain default-off (unset env = current fast behavior) so the
    normal suite path stays ~0.7 s.
  - Don't "fix" by widening the window to a huge constant without wiring the delay env in:
    the regression then passes but the suite pays the cost on every run.
  - `go vet` clean; no debug prints (arch rule R5).
- **Constraints**: path ownership — `shell` scope owns `internal/appshell`
  (`plan/parallel-work-and-versioning.md` §2.5); this branch already owns it for B04.
  Verify with `make verify-bug B=B04` from this worktree in phase 3 (runs L1/L2/L3 incl.
  `make check`).

## Out of scope (→ new bug entries)

Related defects noticed while investigating. File each as its own phase-1 entry; do not
fix here.

- None found. (The parent-side log-rotation trade-off documented at
  `internal/appshell/daemon.go:128-129` — child bytes keep flowing to the renamed file
  after rotation — is a known, commented design trade-off, not a defect.)

## Exit checklist

- [x] Regression test fails on unfixed code for the right reason (deadline exhaustion at
      ~5.0 s, exit 1, 2/2 runs — not a compile error)
- [x] `regression {file, cmd}` recorded in `bug_list.json`
- [x] This doc committed on `fix/B04-flaky-daemon-log-poll`
- [ ] Summary comment posted on the issue
- [ ] `make bug-state B=B04 S=diagnosed` passed (gate re-runs the failing test)
