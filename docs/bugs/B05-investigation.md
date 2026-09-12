# Bug Investigation — B05: TestSearch_sinceUntil_filtering date bomb

- **Bug ID**: B05 · **Issue**: #73 · **Branch**: `fix/B05-search-until-datebomb`
- **Date**: 2026-09-12 · **Phase**: 2 (investigation)
- **Discovered during**: F10 (webui-dev-mock) baseline `make check` — red at session start on
  `feature/frontend-mock-mode` @ af6b33b (= main tip, zero diff), so this predates any F10 work.
  Note: the red was masked for a day because `go test ./...` served cached "ok" results until
  something invalidated the test cache; the failing run at 2026-09-12 14:12 (first baseline check
  of the day) failed for real.

## Symptom

`make check` fails in `internal/httpserver`:

```
--- FAIL: TestSearch_sinceUntil_filtering (0.02s)
    recall_test.go:80: until: got []
```

Deterministic: 5/5 failures solo (`-count=5`), not a load flake. `internal/appshell`
(TestSpawnedDaemonStdioIsLogFdNotPipe) also flaked once the same morning under full-suite load —
that is the known B04 (#70), passes solo, untouched here.

## Root cause

`internal/httpserver/recall_test.go` seeds two memories:

- `rmb://entities/new` at `nowMS` (`time.Now().UTC().UnixMilli()`)
- `rmb://entities/old` at `nowMS - 40*24h`

and then asserts (line 80):

```go
if uris := get(base + "&until=2026-08-02"); len(uris) != 1 || uris[0] != "rmb://entities/old" {
```

`until` is parsed by `recall.ParseTimeValue` (`internal/recall/window.go`) with layout
`"2006-01-02"` in the server's local location → midnight of 2026-08-02. The assertion therefore
requires `now-40d < 2026-08-02T00:00 < now`, i.e. it could only pass while
`now ∈ (2026-08-02, 2026-09-11]`. On 2026-09-12, `now-40d = 2026-08-03T00:00+08:00` falls **after**
the cutoff, so the `until` filter excludes both rows → `got []`. The test hard-coded a date
instead of deriving it from its own clock — a time bomb with a 40-day fuse set around 2026-08-25.

## Regression (fails on unfixed code)

`regression.cmd` in `bug_list.json`:

```
CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/httpserver/ -run TestSearch_sinceUntil_filtering -count=1
```

Verified failing 5/5 on unfixed code at 2026-09-12 14:40 (matches the `→ diagnosed` gate, which
requires the regression to FAIL pre-fix). The test file carries the `// B05` tag.

## Fix direction (for the phase-3 session)

Test-only, one line — derive the cutoff from the test clock, keeping ≥10 days of margin on both
sides of the boundary assertions:

```go
until := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
if uris := get(base + "&until=" + until); len(uris) != 1 || uris[0] != "rmb://entities/old" {
    t.Fatalf("until: got %v", uris)
}
```

old@-40d < cutoff@-30d (midnight) < new@now holds for every date. No product code changes;
`recall.ParseTimeValue` behaves as documented.
