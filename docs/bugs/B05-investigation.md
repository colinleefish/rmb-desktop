# Bug Investigation — B05: flaky TestIncumbentMerge_CosinePath materiality read-skew

- **Bug ID / Issue**: B05 / #72
- **Investigator**: cursor agent (fix/B05-incumbent-materiality-race)
- **Date**: 2026-09-12

## TL;DR — root cause in one line

`bucketUnchanged` reads the incumbent URI's `source_atom_hash` outside any transaction while another bucket goroutine can `mergeIntoIncumbent` on that same URI, so the incumbent bucket falsely thinks evidence changed and re-distills to an extra version bump.

## Evidence chain

1. **CI flake (ubuntu, loaded runner)**
   ```
   TestIncumbentMerge_CosinePath: incumbent must bump to v2, got 3
   log: single "l3 merged new subject into incumbent" (variant path only)
   ```
   → Second bump came from the incumbent bucket's persist path in the same rollup, not a second merge.

2. **Probabilistic local repro**
   ```
   $ CGO_ENABLED=1 go test -tags sqlite_fts5 -count=50 ./internal/worker/memory/ -run TestIncumbentMerge_CosinePath
   ```
   → Occasional `got 3` under parallel bucket goroutines (`l3Concurrency=8`).

3. **Code path**
   - Concurrent bucket loop: `internal/worker/memory/worker.go` (~194–262)
   - Materiality read (no tx): `bucketUnchanged` (~415–447) — `SELECT ... WHERE uri = ? AND superseded_at IS NULL`
   - Variant write: `persistMemory` → `mergeIntoIncumbent` (~705+) supersede + insert on **incumbent URI**
   - `recordingDistiller` returns distinct bodies `b-N` per call, so a false materiality miss always produces a version bump.

**Flaky interleaving**: variant merge commits → incumbent materiality SELECT sees variant's `source_atom_hash` (different atom IDs, same facts) → hash mismatch → distill → body differs → supersede to v3.

## Root cause

- **Defect location**: `internal/worker/memory/worker.go` — `bucketUnchanged` (materiality SELECT) vs concurrent `mergeIntoIncumbent` on the same memory URI
- **What is wrong**: L3 rollup processes buckets in parallel with no per-memory-URI serialization; materiality is evaluated on a non-transactional read that can skew relative to a concurrent incumbent merge.
- **Why it produces the symptom**: The incumbent bucket (`rmb://preferences/redis-credentials-storage`) still runs in the same rollup as the cosine variant (`redis-secrets`). If the variant merge wins the race, the incumbent bucket's gate compares its atom-id fingerprint against the post-merge row, fails open, re-distills (new stub body), and bumps v2→v3 while CI expects only the merge bump.

## Regression test(s)

| Test | File | Expected (post-fix) | Actual (pre-fix, observed) |
|---|---|---|---|
| `TestB05_incumbentMaterialityReadSkew` | `internal/worker/memory/b05_regression_test.go` | incumbent stays v2 | v3 (`B05: ... got 3`) |

Run: `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/worker/memory/ -run TestB05_incumbentMaterialityReadSkew`

Deterministic hook (`rollupTestHook`, `BypassURILock`) forces variant merge commit before the incumbent bucket's materiality SELECT — same skew as CI without `-count` stress.

## Fix direction (not a patch)

- Suggested approach: per-memory-URI mutex from materiality through persist; skip further work on a URI once another bucket in the same rollup batch has already written it (variant merge marks incumbent URI).
- Files to touch: `worker.go`, small `rollup_sync.go` helper, `b05_regression_test.go`, test hooks in `rollup_testhook.go`.
- Risks / edge cases: deadlock if hooks hold locks in tests — B05 test bypasses URI locks intentionally; production always locks.
- Constraints: no change to embedder threshold or atom-id fingerprint semantics in this bug.

## Out of scope (→ new bug entries)

- Using content-based fingerprints across slug variants (separate design change).

## Exit checklist

- [x] Regression test fails on unfixed code for the right reason (not a compile error)
- [x] `regression {file, cmd}` recorded in `bug_list.json`
- [x] This doc committed on `fix/B05-incumbent-materiality-race`
- [ ] Summary comment posted on the issue
- [ ] `make bug-state B=B05 S=diagnosed` passed (gate re-runs the failing test)
