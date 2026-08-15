# T2 pipeline failures (Aug 2026)

Post-incident notes for L2 (scene building) stalls and failures observed on a remote
`rmb-desktop` deployment with ~111 sessions, zero scenes written, while a local install
with hundreds of sessions worked normally.

## Symptoms

- `pipeline-health` shows T1 complete, **T2 never completes** (`t2_done: 0`, `scenes: 0`)
- L2 worker appears stuck on `llm.build_scenes` or fails quickly
- Debug logs may show one or more of:
  - `decode llm response: unexpected end of JSON input` with `status=200`, `resp_bytes=2049`
  - `context deadline exceeded (Client.Timeout exceeded while awaiting headers)`
  - `clear scene fts: database disk image is malformed`
- `PRAGMA integrity_check` on `rmb.db` can still return `ok`
- `pipeline_state` may contain many rows stuck in `l2_status=running` after restarts

## Root causes (layered)

Several independent issues stacked on top of each other. Fix them in order.

### 1. LLM response truncation (0.1.19 regression) — fixed in 0.1.20

In **0.1.19**, `internal/llm/openai_client.go` read **all** HTTP responses through a
2 KiB `LimitReader`, including successful `200` bodies. L2 JSON payloads often exceed
2 KiB, producing:

```
decode llm response: unexpected end of JSON input
resp_bytes: 2049
status: 200
```

**0.1.17 / 0.1.18** streamed the full body on success. **0.1.20** restores that behavior
(only error snippets are capped for logging).

### 2. FTS5 sync bug in `persistScenes` — fixed by trigger-based sync (migration 00009)

**This was initially misdiagnosed as FTS5 index corruption under concurrent writes. It is not corruption.** The `database disk image is malformed` error reproduces on a pristine, single-writer database with `PRAGMA integrity_check` returning `ok`.

Root cause: `persistScenes` managed `scenes_fts` by hand, issuing an FTS5 external-content `'delete'` command for every upserted scene:

```sql
INSERT INTO scenes_fts(scenes_fts, rowid) VALUES('delete', ?)
```

Two FTS5 external-content behaviors combine to break this:

- **First-time scene creation:** the rowid already exists in `scenes` (just upserted) but was never indexed in `scenes_fts`. Deleting such a rowid makes FTS5 return `SQLITE_CORRUPT` (`database disk image is malformed`). `persistScenes` wrapped this in `fmt.Errorf("clear scene fts: %w", err)` and returned it, rolling back the whole transaction — so the scene was never written. This is the exact log symptom.
- **Updates:** the `'delete'` command for an external-content table requires the column values to be supplied explicitly (it does **not** read the content table). The old code passed only the rowid, so the delete was a silent no-op and stale terms accumulated.

The `scenes_fts_data` "2 rows" seen during diagnostics is the **normal empty-FTS5 header state** (`X'000000'` structure + averaging records), not orphan data. `atoms_fts` and `memories_fts` show the same and are healthy.

**Fix (migration `00009_scenes_fts_triggers.sql`):** `scenes_fts` is now kept in sync by `AFTER INSERT/UPDATE/DELETE` triggers on `scenes` (the UPDATE trigger fires only when `abstract`/`body` change, so embedding writes don't reindex). `persistScenes` no longer touches FTS at all — it just upserts and prunes scene rows. This fixes first-create, stale-term-on-update, and orphan-entries-on-prune in one stroke.

Because the index was never actually corrupt, the database file does **not** need FTS repair — only the code upgrade (and resetting zombie `pipeline_state` rows, see Repair).

### 3. Zombie `pipeline_state` rows

After crashes or forced restarts, sessions can remain `l2_status=running` with a stale
`l2_started_at`. Workers treat this as in-flight work, which blocks progress and can
drive concurrency/backpressure incorrectly.

Reset via debug API or SQL (see Repair below).

### 4. LLM gateway timeouts

On slower paths to `stargate.youxi123.com`, `llm.timeout` (default **45s**) may be too
short for `build_scenes`, producing header/body deadline errors unrelated to SQLite.

## Diagnostics

```bash
BASE=http://127.0.0.1:19019/api/v1

curl -sS "$BASE/version"
curl -sS "$BASE/browse/pipeline-health" | jq .
curl -sS "$BASE/debug/workers" | jq .
curl -sS "$BASE/debug/in-flight" | jq .
curl -sS "$BASE/debug/logs?tail=50&level=warn" | jq .
curl -sS "$BASE/debug/pipeline/stuck?older_than=2m" | jq .
curl -sS "$BASE/debug/sqlite" | jq .
```

On the database file (quit RMB Desktop first):

```bash
DB=~/Library/Application\ Support/rmb-desktop/data/rmb.db

sqlite3 "$DB" "PRAGMA integrity_check;"
sqlite3 "$DB" "PRAGMA journal_mode;"
sqlite3 "$DB" "SELECT l2_status, COUNT(*) FROM pipeline_state GROUP BY l2_status;"
sqlite3 "$DB" "SELECT COUNT(*) FROM scenes; SELECT COUNT(*) FROM scenes_fts;"
sqlite3 "$DB" "SELECT name FROM sqlite_master WHERE name='scenes_fts';"
```

Interpretation:

| Log / check | Meaning |
|-------------|---------|
| `resp_bytes: 2049` + JSON decode error | 0.1.19 truncation → upgrade to **≥ 0.1.20** |
| `deadline exceeded` | raise `llm.timeout` or reduce load |
| `clear scene fts: ... malformed` | FTS5 sync bug in `persistScenes` (not real corruption) → upgrade to a build with migration `00009` (trigger-based FTS sync) |
| many `l2_status=running` | zombie state → reset to `pending` |
| `integrity_check` ok but FTS delete ops fail | not corruption — the FTS5 external-content `'delete'` was misused (see root cause #2); fixed by migration `00009` triggers |

## Repair

**Requirements:** RMB Desktop **≥ 0.1.20** (LLM truncation fix) **and** a build that includes migration `00009` (trigger-based `scenes_fts` sync). App fully quit.

The database file itself is **not corrupt** (`PRAGMA integrity_check` returns `ok`), so the FTS rebuild/DROP steps from earlier drafts are unnecessary. The only data-level action needed is clearing zombie/stuck `pipeline_state` rows so workers retry them:

```bash
DB=~/Library/Application\ Support/rmb-desktop/data/rmb.db
cp "$DB" "$DB.bak.$(date +%s)"

sqlite3 "$DB" <<'SQL'
-- Reset stuck / failed T2 rows so L2 retries them.
UPDATE pipeline_state
SET l2_status='pending', l2_started_at=NULL, l2_last_error=NULL
WHERE l2_status IN ('running', 'failed');
SQL
```

On next launch the app runs migration `00009` (creates the `scenes_fts` sync triggers) and `rebuildFTSIndexes` reconciles the FTS index from the `scenes` content table. No `l2_max_concurrency` reduction is required — the original concurrency theory was a misdiagnosis.

Optionally reset stale `running` rows without SQL via `POST /api/v1/debug/pipeline/unstick`.

Then in `config.yaml` (only if LLM gateway timeouts persist, unrelated to SQLite):

```yaml
llm:
  timeout: 120s   # optional, only if build_scenes header timeouts recur
```

## Verification

After repair and upgrade:

1. `GET /api/v1/browse/pipeline-health` — `t2_done` should increase; `scenes` count > 0
2. Debug logs should show `l2 built scenes` without `malformed`
3. LLM traces should show `resp_bytes` well above 2048 on successful `build_scenes`

Dry-run (no persist) for a single session:

```bash
curl -sS -X POST "$BASE/debug/pipeline/dry-run" \
  -H 'Content-Type: application/json' \
  -d '{"session_key":"<uuid>","stage":"t2"}'
```

## Related code

| Area | Path |
|------|------|
| LLM response read | `internal/llm/openai_client.go` |
| L2 persist | `internal/worker/scene/worker.go` (`persistScenes`) — FTS now handled by triggers, not manually |
| FTS schema | `internal/db/migrations/00002_distillation.sql` |
| FTS sync triggers (fix) | `internal/db/migrations/00009_scenes_fts_triggers.sql` |
| Startup FTS rebuild | `internal/worker/runner.go` (`rebuildFTSIndexes`) |
| L2 concurrency | `internal/worker/backpressure/controller.go`, `pipeline.l2_max_concurrency` |
| Debug endpoints | `internal/httpserver/debug.go` |

## Timeline

| Version | Issue |
|---------|-------|
| ≤ 0.1.18 | T2 slow / timeout on some networks; otherwise healthy |
| 0.1.19 | 2 KiB response cap breaks most L2 LLM calls |
| 0.1.20 | LLM read fixed; `persistScenes` FTS sync bug still blocks first-time scene creation (`clear scene fts: malformed`) |
| +migration 00009 | `scenes_fts` synced via triggers; `persistScenes` no longer touches FTS — first-create, stale-term, and prune-orphan issues all resolved |
