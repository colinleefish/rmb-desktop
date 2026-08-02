# M0 spike results

Date: 2026-08-02

## SQLite driver

**Choice:** `mattn/go-sqlite3` + `sqlite-vec-go-bindings/cgo` (CGO)

**Why:** ncruces WASM bindings hit version skew with latest go-sqlite3 (M0). CGO path works on macOS/Linux dev and CI with a C toolchain. Revisit WASM for static Windows builds later.

**Verified by:** `CGO_ENABLED=1 go test -tags sqlite_fts5 ./internal/spike/...`

**Build tag:** `sqlite_fts5` required (macOS system SQLite lacks FTS5).

## FTS5

- `unicode61` — EN + CJK keyword queries pass in spike test
- `trigram` — substring match (`Kube` → `Kubernetes`) passes

## RRF

- `internal/recall/rrf.go` — 70/30 vector-primary fusion prototype passes unit test

## Hook loop

- `rmb hook-submit --source=cursor` → `POST /api/v1/sessions/:id/upload` → row in `session_turns`
- Verified by: `go test ./internal/httpserver/...` and manual smoke (`scripts/smoke.sh`)

## Migrations

**Choice:** goose (embedded SQL in `internal/db/migrations/`)

## Timestamps

**Choice:** INTEGER unix milliseconds UTC

## Open after M0

- [ ] Default embed model doc: BGE-M3 vs multilingual-e5 (pick at M3)
- [ ] Trigram FTS in production schema: include in M1 atoms/memories FTS tables
