# Simplification opportunities — 2026-08-14

This report is the output of a read-only code-simplification scan of
`rmb-desktop`. It covers the recently changed pipeline code
(`internal/worker/scene`, `internal/db`, `internal/worker/backpressure`,
`internal/worker/memory`, `internal/worker/extract`, `internal/worker/runner.go`)
and a broader survey of `internal/`. No source file was modified. All findings
are behavior-preserving simplifications (dead-code removal, de-duplication, or
local clarity fixes); the risk column indicates how much surrounding change is
needed.

| Risk     | Findings |
|----------|----------|
| low      | 11       |
| medium   | 5        |
| high     | 1        |
| **total**| **17**   |

---

## Low risk

### L1. `joinPlaceholders` re-implements `strings.Join`
- **File:** `internal/worker/scene/worker.go:431-439` (used at `:385`)
- **Current code:**
  ```go
  func joinPlaceholders(parts []string) string {
      out := ""
      for i, p := range parts {
          if i > 0 {
              out += ","
          }
          out += p
      }
      return out
  }
  ```
- **Suggestion:** delete the helper and call `strings.Join(placeholders, ",")` at
  the call site. The same pattern is already used in
  `internal/browse/browse.go:917`.
- **Why safe:** `joinPlaceholders` is exactly `strings.Join` with `","`; all
  inputs are `"?"`, so the generated SQL is byte-identical.
- **Risk:** low

### L2. `DryRunT2` serializes each chunk twice
- **File:** `internal/worker/scene/dryrun.go:108-130`
- **Current code:**
  ```go
  if err := step("serialize_"+chunkName, func() (string, error) {
      raw, err := serializeAtomsForLLM(chunk)
      ...
      return fmt.Sprintf("%d bytes", len(raw)), nil
  }); err != nil { ... }

  var llmRaw string
  if err := step("llm.build_scenes_"+chunkName, func() (string, error) {
      atomsJSON, err := serializeAtomsForLLM(chunk) // second call
      ...
  ```
- **Suggestion:** serialize once before the first step, keep `atomsJSON`, reuse
  its length for the trace detail and pass the value to `BuildScenes`.
- **Why safe:** `serializeAtomsForLLM` is deterministic (JSON marshalling of the
  same struct); the trace output and the LLM payload are unchanged.
- **Risk:** low

### L3. `chunkGroups` empty case returns a one-element chunk
- **File:** `internal/worker/scene/group.go:76-78`
- **Current code:**
  ```go
  if len(groups) == 0 {
      return [][]atomGroup{groups}
  }
  ```
- **Suggestion:** return `nil` (or an empty slice) instead of
  `[][]atomGroup{nil}`.
- **Why safe:** both call sites (`worker.go:198`, `dryrun.go:102`) are already
  guarded so `groups` is non-empty (a non-empty atom list always produces at
  least one group), so the empty branch is unreachable. The change removes a
  misleading edge case rather than altering observed behavior.
- **Risk:** low

### L4. `equalStringSets` is more machinery than needed
- **File:** `internal/worker/memory/group.go:154-173`
- **Current code:** builds two `map[string]struct{}` sets, then compares lengths
  and membership.
- **Suggestion:** use a single counting map:
  ```go
  func equalStringSets(a, b []string) bool {
      if len(a) != len(b) { return false }
      counts := make(map[string]int, len(a))
      for _, s := range a { counts[s]++ }
      for _, s := range b {
          if counts[s] == 0 { return false }
          counts[s]--
      }
      return true
  }
  ```
- **Why safe:** equivalent set semantics (including duplicate elements), shorter
  and single-pass over `b`.
- **Risk:** low

### L5. `backpressure.Outcome.Pressure` duplicates `Err` in production
- **Files:** `internal/worker/backpressure/controller.go:7-16`,
  `internal/worker/scene/worker.go:121-124`,
  `internal/worker/extract/worker.go:123-126`
- **Current code:**
  ```go
  w.bp.Observe(backpressure.Outcome{
      Err:      err,
      Pressure: llm.IsTransientError(err),
  })
  ```
  and in the controller:
  ```go
  func (o Outcome) hasPressure() bool {
      return o.Pressure || llm.IsTransientError(o.Err)
  }
  ```
- **Suggestion:** drop `Pressure` from the two production call sites; keep the
  field only as the test seam (`controller_test.go:33` sets it directly).
- **Why safe:** `hasPressure()` already infers pressure from `Err` via
  `llm.IsTransientError`, so the result is identical; only redundant work is
  removed.
- **Risk:** low

### L6. Dead function `snippet160`
- **File:** `internal/recall/fts.go:23-35`
- **Current code:**
  ```go
  func snippet160(abstract, body sql.NullString) string { ... }
  ```
- **Suggestion:** delete it.
- **Why safe:** `rg -w snippet160` shows only its definition; the FTS paths build
  snippets inline in SQL.
- **Risk:** low

### L7. Dead function `anyPathExists`
- **File:** `internal/setup/files.go:105-112`
- **Suggestion:** delete it (and, if it becomes unused, its helper `pathExists`
  — but `pathExists` is still used in `detect.go` and `claude.go`).
- **Why safe:** no references anywhere (`rg -w anyPathExists` returns only the
  definition).
- **Risk:** low

### L8. Dead function `homePaths`
- **File:** `internal/setup/agent.go:91-100`
- **Suggestion:** delete it.
- **Why safe:** no references anywhere.
- **Risk:** low

### L9. Dead function `redactSecretsInPreview`
- **File:** `internal/setup/detect.go:41-48`
- **Suggestion:** delete it. (`redactSettingsJSON`, which it delegates to, is
  still used elsewhere.)
- **Why safe:** no references anywhere; `redactSettingsJSON` remains covered by
  tests.
- **Risk:** low

### L10. Empty package directory `internal/seed/`
- **File:** `internal/seed/` (contains no `.go` files)
- **Suggestion:** remove the empty directory from the repo.
- **Why safe:** Go ignores empty package directories; deleting it changes
  nothing at build time.
- **Risk:** low

### L11. `gofmt` inconsistency in `buildModelsURLCandidates`
- **File:** `internal/llm/models_list.go:39`
- **Current code:**
  ```go
  func buildModelsURLCandidates(baseURL string) []string {
  trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
  ```
- **Suggestion:** indent the statement (run `gofmt -w`).
- **Why safe:** purely formatting; `go vet`/`go build` already accept the file,
  and gofmt does not change semantics.
- **Risk:** low

---

## Medium risk

### M1. Code-fence stripping is copy-pasted into three parsers
- **Files:**
  - `internal/worker/scene/parse.go:32-43`
  - `internal/worker/memory/parse.go:20-31`
  - `internal/worker/extract/parse.go:25-36`
- **Current code (identical in all three):**
  ```go
  raw = strings.TrimSpace(raw)
  if raw == "" { return ..., fmt.Errorf("empty llm response") }
  if strings.HasPrefix(raw, "```") {
      lines := strings.Split(raw, "\n")
      if len(lines) >= 2 {
          end := len(lines)
          if strings.TrimSpace(lines[end-1]) == "```" {
              end--
          }
          raw = strings.Join(lines[1:end], "\n")
      }
  }
  ```
- **Suggestion:** extract one package-level helper (e.g. in `internal/llm`):
  ```go
  func StripCodeFence(raw string) (string, error) {
      raw = strings.TrimSpace(raw)
      if raw == "" { return "", errors.New("empty llm response") }
      ...
      return raw, nil
  }
  ```
  and have each parser call it.
- **Why safe:** the byte-for-byte transformation is identical in all three
  places; moving it into one function preserves output for every input.
- **Risk:** medium (touches three packages; needs the existing parse tests to
  stay green)

### M2. `nullIfEmpty` is defined twice
- **Files:**
  - `internal/pipeline/state.go:132-138`
  - `internal/session/upload.go:148-153`
- **Current code:** two near-identical helpers (pipeline's version also trims
  whitespace).
- **Suggestion:** keep one shared helper (prefer the pipeline version, which
  trims) in a shared low-level package and have `session` import it; or unify
  both call sites to `pipeline.nullIfEmpty`.
- **Why safe:** for the current inputs the only behavioral difference is
  whitespace trimming, which is harmless and arguably more correct; if exact
  parity is desired, use the trimming version everywhere.
- **Risk:** medium

### M3. Atom-row scanning is duplicated between scene and memory workers
- **Files:**
  - `internal/worker/scene/worker.go:290-316` (`loadSessionAtoms`)
  - `internal/worker/memory/worker.go:443-466` (`scanAtoms`)
- **Current code:** both `Scan` the same 10 columns, handle `scene_name`/`slug`
  `sql.NullString` the same way, and call `db.UnmarshalStringArray` on
  `source_turn_ids`.
- **Suggestion:** introduce one shared row scanner, e.g.
  `scanAtom(row scanner) (model.Atom, error)`, parameterized over `*sql.Rows`
  vs `*sql.Tx` (or use a small `interface { Scan(...any) error }`), then have
  both loaders use it.
- **Why safe:** the column set and conversion logic are identical; consolidating
  them removes the drift risk (e.g. the redundant `var err error` in
  `scanAtoms`) without changing the loaded data.
- **Risk:** medium

### M4. `handleProcessError` is duplicated between extract and scene workers
- **Files:**
  - `internal/worker/extract/worker.go:425-432`
  - `internal/worker/scene/worker.go:421-428`
- **Current code (identical except the `StageL1`/`StageL2` constant):**
  ```go
  if llm.IsTransientError(cause) {
      w.log.Warn("l1 transient error", ...)
      _ = pipeline.MarkPending(ctx, w.db, sessionID, pipeline.StageL1, cause.Error(), w.now())
      return cause
  }
  _ = pipeline.MarkFailed(ctx, w.db, sessionID, pipeline.StageL1, cause.Error(), w.now())
  return cause
  ```
- **Suggestion:** extract a shared helper (e.g. in `internal/worker` or
  `internal/pipeline`) taking the stage, logger tag, and error; both workers
  call it.
- **Why safe:** the two bodies are line-for-line identical aside from the stage
  constant; parameterizing that constant preserves behavior exactly.
- **Risk:** medium

### M5. Worker `Run` ticker loop is copy-pasted across four workers
- **Files:**
  - `internal/worker/extract/worker.go:54-77`
  - `internal/worker/scene/worker.go:52-76`
  - `internal/worker/memory/worker.go:47-75`
  - `internal/worker/embed/worker.go:39-60`
- **Current code (same shape everywhere):**
  ```go
  w.reg.WorkerStarted("l1")
  defer w.reg.WorkerStopped("l1")
  w.log.Info("l1 extract worker started", "poll_interval", interval)
  w.runOneCycle(ctx)
  ticker := time.NewTicker(interval)
  defer ticker.Stop()
  for {
      select {
      case <-ctx.Done(): ... return nil
      case <-ticker.C: w.runOneCycle(ctx)
      }
  }
  ```
- **Suggestion:** add one small loop helper, e.g.
  `internal/worker/runPoll(ctx, name, interval, runOneCycle func(context.Context)) error`,
  and have each worker call it after its own setup/logging.
- **Why safe:** the loop semantics (immediate first cycle, ticker cadence, stop
  on context cancel) are identical in all four workers; consolidating them
  removes only repetition.
- **Risk:** medium (four call sites; the embed worker's cycle is already
  simplified so it is the lowest-risk migration target)

---

## High risk

### H1. The L1/L2 backpressure cycle is duplicated wholesale
- **Files:**
  - `internal/worker/extract/worker.go:84-145` (`runOneCycle`)
  - `internal/worker/scene/worker.go:82-143` (`runOneCycle`)
- **Current code (identical structure):** select candidates → seed concurrency
  from backlog → log cycle → loop over `remaining` in `limit`-sized batches →
  `backpressure.RunParallel` with an `Observe` wrapper → `EndCycle(pendingHint)`
  → early-return when concurrency drops.
- **Suggestion:** extract a generic
  `runBackpressuredCycle(ctx, bp, reg, stage, selectCandidates, countPending, processSession)` 
  shared between the two workers. The only real differences are:
  - the worker name / log strings (`"l1"` vs `"l2"`)
  - `selectCandidateSessions`'s query and fallback limit
  - the `countPendingSessions` query
- **Why safe:** the two methods are line-for-line identical except for the stage
  name and the two SQL queries, which would be passed in as parameters. This is
  the highest-value de-duplication in the worker tree, but it touches the
  concurrency/backpressure hot path.
- **Risk:** high (needs the existing `pipeline_test.go` / backpressure tests plus
  a manual L1/L2 soak run to confirm identical scheduling behavior)
