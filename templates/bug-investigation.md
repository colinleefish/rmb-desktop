# Bug Investigation — B<nn>: <title>

> **Phase 2 artifact** (AGENTS.md "Bug workflow"). This doc is the fix agent's ONLY context —
> write it so they never re-investigate. Commit as `docs/bugs/B<nn>-investigation.md` on the
> bug's `fix/B<nn>-<slug>` branch (never directly on main — a failing test on main breaks the
> green baseline). When done:
> 1. Record `regression = {file, cmd}` on the bug entry in `bug_list.json`
> 2. Post a one-paragraph summary comment on the issue
> 3. `make bug-state B=<nn> S=diagnosed` — the gate runs your regression test and
>    **requires it to FAIL on unfixed code**. A passing test means the bug was not reproduced.

- **Bug ID / Issue**: B<nn> / #<n>
- **Investigator**: <agent id / session>
- **Date**: YYYY-MM-DD

## TL;DR — root cause in one line

One sentence, pointing at code. Example: "`internal/hook/zcode.go:142` skips `role=user`
transcript entries, so user prompts never reach the distiller."

## Evidence chain

Every step: observation → command → key output → conclusion (`file:line`).
The fix agent must be able to re-run each step.

1. **<observation>**
   ```
   $ <command>
   <key output>
   ```
   → <conclusion at file:line>
2. ...

**Flaky bugs**: replace the single repro with statistics — N runs, failure rate, and the
interleaving/timing window that triggers it.

## Root cause

- **Defect location**: `file:line`
- **What is wrong**: ...
- **Why it produces the symptom**: the causal chain, closed from code to the user-visible
  behavior reported in the bug report

## Regression test(s)

Committed in this phase; each test tagged with a `// B<nn>` comment. Expected to FAIL
before the fix, PASS after. Recorded in `bug_list.json` as `regression = {file, cmd}`.

| Test | File | Expected (post-fix) | Actual (pre-fix, observed) |
|---|---|---|---|
| `TestXxx` | `internal/.../x_test.go` | ... | ... |

Run: `<the exact regression cmd>`

## Fix direction (not a patch)

- Suggested approach: ...
- Files to touch: ...
- Risks / edge cases: ...
- Constraints: arch rules (`.harness/arch-rules.json`), path ownership (`plan/parallel-work-and-versioning.md` §2.5), perf limits

This section deliberately stops short of a diff — implementation choices belong to the fix agent.

## Out of scope (→ new bug entries)

Related defects noticed while investigating. File each as its own phase-1 entry; do not fix here.

- ...

## Exit checklist

- [ ] Regression test fails on unfixed code for the right reason (not a compile error)
- [ ] `regression {file, cmd}` recorded in `bug_list.json`
- [ ] This doc committed on `fix/B<nn>-<slug>`
- [ ] Summary comment posted on the issue
- [ ] `make bug-state B=<nn> S=diagnosed` passed (gate re-runs the failing test)
