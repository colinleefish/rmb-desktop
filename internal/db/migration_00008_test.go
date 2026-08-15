package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigration00008_PurgesMultipleAgentMemoriesWithoutFTSCorrupt verifies the
// fixed 00008 against the scenario that blocked the real upgrade: a v7 database
// with two rmb://agent memory versions (one superseded, one active) indexed in
// memories_fts.
//
// The original 00008 issued a multi-row
//
//	DELETE FROM memories_fts WHERE rowid IN (SELECT rowid FROM memories WHERE category = 'agent');
//
// on the external-content memories_fts table. On the affected long-lived
// database (and on SQLite >= 3.51 after a segment merge) this returns
// SQLITE_CORRUPT ("database disk image is malformed"), aborting the migration
// and preventing the daemon from starting. The fixed 00008 deletes the base
// rows first and rebuilds the index instead.
//
// Note: the bundled mattn/go-sqlite3 SQLite does not reproduce the malformed
// for this small synthetic index, so this test primarily locks in the fixed
// migration's correctness (purge + consistent FTS + integrity) rather than
// failing on the old SQL in CI. The malformed was confirmed manually against
// the real database and the system sqlite3 3.51 CLI.
func TestMigration00008_PurgesMultipleAgentMemoriesWithoutFTSCorrupt(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	// Seed two rmb://agent memory versions (one superseded, one active) — exactly
	// the state observed on the affected v7 database — each indexed in
	// memories_fts in separate transactions. The shared uri is allowed because
	// only the active row has superseded_at IS NULL (see idx_memories_active_uri).
	if err := seedAgentMemoryRow(database, "agent-v1", 1, 1); err != nil { // superseded
		t.Fatalf("seed agent-v1: %v", err)
	}
	if err := seedAgentMemoryRow(database, "agent-v2", 2, 0); err != nil { // active
		t.Fatalf("seed agent-v2: %v", err)
	}

	// Sanity: two agent rows, both indexed.
	if got := count(t, database, `SELECT COUNT(*) FROM memories WHERE category='agent'`); got != 2 {
		t.Fatalf("precondition: agent memories = %d, want 2", got)
	}
	if got := count(t, database, `SELECT COUNT(*) FROM memories_fts`); got != 2 {
		t.Fatalf("precondition: memories_fts rows = %d, want 2", got)
	}

	// Force an FTS5 segment merge. On a long-lived database the index accumulates
	// and auto-merges segments over time; a multi-row
	// `DELETE FROM memories_fts WHERE rowid IN (...)` against a merged external-
	// content index returns SQLITE_CORRUPT ("database disk image is malformed").
	// A fresh single-segment index does not reproduce this, so we merge explicitly
	// to mirror the real upgrade-time state.
	if _, err := database.Exec(`INSERT INTO memories_fts(memories_fts) VALUES('optimize')`); err != nil {
		t.Fatalf("optimize memories_fts: %v", err)
	}

	// Roll goose back to v7 so the next migrate() re-applies 00008 (and 00009).
	if _, err := database.Exec(`DELETE FROM goose_db_version WHERE version_id >= 8`); err != nil {
		t.Fatalf("roll back goose version: %v", err)
	}

	// Re-run migrations. The old 00008 failed here with SQLITE_CORRUPT.
	if err := migrate(database); err != nil {
		t.Fatalf("re-migrate with two agent memories: %v (old 00008 hit SQLITE_CORRUPT here)", err)
	}

	// 00008 must have purged all agent memories.
	if got := count(t, database, `SELECT COUNT(*) FROM memories WHERE category='agent'`); got != 0 {
		t.Fatalf("agent memory count after 00008 = %d, want 0", got)
	}

	// memories_fts must be consistent with the remaining memories and the
	// database must pass integrity_check (no lingering FTS corruption).
	mem := count(t, database, `SELECT COUNT(*) FROM memories`)
	fts := count(t, database, `SELECT COUNT(*) FROM memories_fts`)
	if fts != mem {
		t.Fatalf("memories_fts rows = %d, want %d (matching memories)", fts, mem)
	}

	var ic string
	if err := database.QueryRow(`PRAGMA integrity_check`).Scan(&ic); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if ic != "ok" {
		t.Fatalf("integrity_check = %q, want ok", ic)
	}

	// 00009 must (still) have created the scenes_fts sync triggers.
	if got := count(t, database, `SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND tbl_name='scenes'`); got != 3 {
		t.Fatalf("scenes triggers after re-migrate = %d, want 3", got)
	}
}

// seedAgentMemoryRow inserts one rmb://agent memory with the given id/version
// and a supersededAt of 0 meaning "active" (NULL) when zero, else superseded,
// then indexes it in memories_fts — mirroring how the memory worker persists a
// memory (insert-only, no triggers at this schema version).
func seedAgentMemoryRow(db *sql.DB, id string, version int, supersededAt int64) error {
	abstract := "agent guide " + id
	body := "agent guide body " + id
	var sup any
	if supersededAt != 0 {
		sup = supersededAt
	}
	if _, err := db.Exec(`
		INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, created_at, updated_at)
		VALUES (?, 'rmb://agent', 'agent', ?, ?, ?, ?, 0, 0)`,
		id, version, sup, abstract, body); err != nil {
		return err
	}
	_, err := db.Exec(`
		INSERT INTO memories_fts (rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`, id, abstract, body)
	return err
}

func count(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}
