# Simplification implementation report — 2026-08-15

Worktree branch: `refactor/simplification`. Implements the findings from
[`simplification-opportunities-2026-08-14.md`](simplification-opportunities-2026-08-14.md).
Every change is behavior-preserving; each finding (or tightly-related batch) is
one commit. Note: tests in this repo must be run with the project's build tags
(`make test`, i.e. `CGO_ENABLED=1 go test -tags sqlite_fts5 ./...`); without
the `sqlite_fts5` tag the FTS5 migrations fail for environmental reasons.

## Summary

| Finding | Status | Commit |
|---------|--------|--------|
| L1  | done | `fbf83b3` Simplify: L1 joinPlaceholders -> strings.Join |
| L2  | done | `f85b86e` Simplify: L2 DryRunT2 serializes each chunk once |
| L3  | **skipped** (see below) | — |
| L4  | done (deviation from report snippet) | `83c376f` Simplify: L4 equalStringSets single map, exact set semantics |
| L5  | done | `7ca5ec4` Simplify: L5 drop redundant Pressure field in production Observe calls |
| L6–L10 | done | `262158e` Simplify: remove dead code (L6 snippet160, L7 anyPathExists, L8 homePaths, L9 redactSecretsInPreview, L10 empty internal/seed dir) |
| L11 | done | `c2a814b` Simplify: L11 gofmt buildModelsURLCandidates |
| M1  | done | `8c32b00` Simplify: M1 shared llm.StripCodeFence for the three LLM parsers |
| M2  | done | `dcadb4c` Simplify: M2 shared db.NullIfEmpty (trimming variant) for pipeline and session |
| M3  | done | `7fb442d` Simplify: M3 shared db.ScanAtomRows for scene and memory workers |
| M4  | done (placement deviation) | `5e989a3` Simplify: M4 shared MarkProcessError in internal/worker/shared |
| M5  | done | `043271e` Simplify: M5 shared RunPoll loop for the four stage workers |
| H1  | done | `bb12d04` Simplify: H1 unified RunBackpressuredCycle shared by L1/L2 workers |

Additionally, commit `be77eca` ("chore: gofmt pre-existing formatting debt")
formats 19 files that were committed unformatted before this branch. The task
requires `gofmt -l .` to pass before every commit, which was impossible with
the pre-existing violations; that standalone whitespace-only commit fixes the
baseline.

## Low risk

- **L1** — `joinPlaceholders` deleted; the call site uses
  `strings.Join(placeholders, ",")`. Generated SQL is byte-identical.
- **L2** — `DryRunT2` serializes each chunk once; the first `serialize_*` step
  captures `atomsJSON` and the `llm.build_scenes_*` step reuses it.
  `serializeAtomsForLLM` is deterministic, so the trace detail ("N bytes") and
  the LLM payload are unchanged.
- **L3 — skipped.** The report claims the `len(groups) == 0` branch of
  `chunkGroups` is unreachable, but `TestChunkGroupsEmpty`
  (internal/worker/scene/group_test.go:69) calls `chunkGroups(nil, 60, 8)`
  and asserts the one-element empty chunk. The branch is therefore *tested,
  documented behavior*, not dead code. Returning `nil` would require changing
  the test's assertion — i.e. changing the function's contract — which is not
  behavior-preserving.
- **L4 — done with a deviation.** The report's snippet implements *multiset*
  equality (duplicates counted), but the old code implements *set* equality
  (duplicates collapsed): `equalStringSets(["x","x"], ["x"])` was `true` and
  would become `false` under the report's version. I kept a single-map
  rewrite that preserves exact set semantics (verified against 7 input
  shapes, including mismatched duplicate counts, with a throwaway table test
  during development).
- **L5** — the two production `Observe` call sites now pass
  `backpressure.Outcome{Err: err}` only; `Outcome.Pressure` remains as the
  test seam (`controller_test.go` sets it directly) and `hasPressure()`
  still infers pressure from `Err` via `llm.IsTransientError`.
- **L6–L9** — dead functions deleted (`snippet160`, `anyPathExists`,
  `homePaths`, `redactSecretsInPreview`) along with their now-unused imports.
  `pathExists` and `redactSettingsJSON` are kept (still referenced). Verified
  with `rg -w` before deletion.
- **L10** — empty `internal/seed/` directory removed (git never tracked it).
- **L11** — `internal/llm/models_list.go` gofmt'd.

## Medium risk

- **M1** — new `llm.StripCodeFence(raw string) (string, error)` in
  `internal/llm/strip.go` (byte-identical transformation and the same
  `"empty llm response"` error string); the scene/memory/extract parsers call
  it. `internal/llm` was chosen per the report and is a leaf in the worker
  dependency graph, so no cycles. Added `strip_test.go` covering fenced /
  unfenced / unterminated-fence / empty inputs.
- **M2** — new `db.NullIfEmpty` in `internal/db/null.go` (the trimming
  variant). `pipeline` uses it via a `rdb` import alias (its `db *sql.DB`
  parameters shadow the package name); `session` imports `internal/db`
  directly. Both local copies deleted. The session call site passes a value
  that is already `strings.ToLower(strings.TrimSpace(...))`-normalized, so the
  trimming difference is unobservable there; pipeline behavior is unchanged.
- **M3** — new `db.ScanAtomRows(rows *sql.Rows) ([]model.Atom, error)` in
  `internal/db/atoms.go`; `internal/db` already owns the column helpers
  (`Marshal/UnmarshalStringArray`) and now imports `internal/model` (model is
  a leaf, no cycle). scene `loadSessionAtoms` and memory `loadAllAtoms` use
  it; memory's private `scanAtoms` is deleted. The `inspect` and `debug`
  atom scans transform rows into different shapes and were left alone.
- **M4 — placement deviation.** The report suggests `internal/worker` or
  `internal/pipeline`, but neither works: package `worker` imports the worker
  subpackages (cycle), and `pipeline` cannot import `llm` because
  `internal/llm`'s *test* file imports `internal/debug`, which imports
  `pipeline` (test-only import cycle). Per the task's guidance a small shared
  package `internal/worker/shared` hosts `MarkProcessError` (and the M5/H1
  helpers). Log messages are composed as `string(stage)+" transient error"`,
  reproducing `"l1/l2 transient error"` exactly; the thin
  `handleProcessError` wrappers remain on both workers.
- **M5** — `shared.RunPoll(ctx, shared.PollOptions{...})` replaces the four
  copy-pasted ticker loops. Options carry the registry name (`"l1"`), a log
  label (`"l1 extract"`), extra start-line attrs (L1/L2 add
  min/max_concurrency after `poll_interval`, preserving attribute order), and
  the cycle function. Lifecycle messages are byte-identical
  (`"<label> worker started/stopped"`); the immediate first cycle, ticker
  cadence, and ctx-cancel semantics are unchanged. Added
  `poll_test.go` covering registration, first-cycle, and stop logging.
  The `time` import was dropped from the embed worker (now unused).

## High risk

- **H1 — done.** The two `runOneCycle` bodies were line-for-line identical
  except for the `"l1"`/`"l2"` strings and one comment (verified with a
  literal diff before extracting). They now delegate to
  `shared.RunBackpressuredCycle(ctx, name, bp, reg, log, CycleDeps{...})`
  where `CycleDeps` supplies `SelectCandidates`, `CountPending`, and
  `ProcessSession`. All log lines, registry calls (`BeginCycle`,
  `SetConcurrency`, `SetBackpressure`), the seed-from-backlog step,
  batch/`RunParallel`/`Observe` loop, `EndCycle(pendingHint)`, and the
  pressure early-exit are preserved verbatim, parameterized by `name`.
  Confidence measures:
  - new `cycle_test.go` pins the unified scheduling semantics: all-batch
    processing with controller bounds, early stop on transient pressure
    (429), empty-candidates short circuit, and select-failure logging;
  - `TestExtractOneCycle` (internal/worker) still exercises the real L1
    worker end-to-end through the shared cycle against a real SQLite DB;
  - worker/extract/scene/backpressure/shared suites re-run with `-count=3`
    as a mini soak; full suite green.

## Verification

Final state of the required checks (run from the worktree root):

- `gofmt -l .` → empty (clean)
- `go vet ./...` → pass
- `go build ./...` → pass
- `go test ./...` → pass for every package when run as the project does
  (`CGO_ENABLED=1 go test -tags sqlite_fts5 ./...`; the plain untagged
  invocation fails in FTS5-dependent packages for environmental reasons that
  predate this branch — the module needs the `sqlite_fts5` CGO tag, see the
  Makefile). `internal/http/static` additionally requires `make webui-build`
  to have produced the embedded web assets; this was run once in the worktree
  to confirm that package passes too (the generated assets are untracked).
