# Clean-State Checklist — a session is not complete until all 5 pass

> Machine-checkable via `make clean-check` (scripts/clean-state-check.sh, idempotent).
> The checklist is the human-readable version; the script is the truth.

1. **Build passes** — `go build` (with sqlite_fts5 tags) succeeds.
2. **Tests pass** — `go test ./...` green (eval gate + check-arch live in `make check`; run that for the full predicate).
3. **State files updated** — `feature_list.json` valid (≤1 active feature; every `passing` feature carries evidence) and `bug_list.json` valid (≤1 bug `fixing`; every `passing` bug carries evidence; states in the five-state machine).
4. **No debug artifacts** — no `.orig/.rej/.swp/.DS_Store` tracked or left in the tree; no `fmt.Print*` outside cmd/ (R5).
5. **Startup path works** — embedded webui present (`internal/http/static/web/index.html`); `bin/` binaries either current or absent (never stale).

Failure on any dimension: fix before committing. If a dimension is red for reasons outside this session's scope, record it as a Blocker in `PROGRESS.md` — do not commit around it silently.
