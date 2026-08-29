package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigration00014_NormalizesZcodeSessionKeys verifies the issue #62
// backfill: zcode sessions that stored ZCode's native ids verbatim
// (sess_<uuid>) are rewritten to the bare-UUID form the normalized parser
// now produces, so old sessions remain reachable and future uploads merge
// into the same rows instead of orphaning them.
//
// Locked-in behaviors:
//   - sess_<uuid>          -> bare UUID (rewritten)
//   - sess_subagent_agent_<uuid> -> left alone (not reducible to a UUID)
//   - already-bare keys    -> untouched
//   - other sources        -> untouched (source='zcode' filter)
//   - target key already exists -> left alone (unique-index collision guard)
//   - statement is idempotent (re-running the migration changes nothing)
func TestMigration00014_NormalizesZcodeSessionKeys(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	// Seed the exact shapes seen in the live database plus the guard cases.
	seed := map[string]string{
		"id-interactive": "sess_f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
		"id-subagent":    "sess_subagent_agent_a6f1b071-e073-411c-81d4-04f787791151",
		"id-alreadybare": "01a0485f-e20b-7fbc-a2e3-36abe88ce1e8",
		"id-cursor":      "sess_1682106d-8aae-404d-aca6-e6581a52b1fb",
		"id-collision":   "sess_11111111-2222-4333-8444-555555555555",
	}
	sources := map[string]string{
		"id-interactive": "zcode",
		"id-subagent":    "zcode",
		"id-alreadybare": "zcode",
		"id-cursor":      "cursor",
		"id-collision":   "zcode",
	}
	for id, key := range seed {
		if err := seedSessionRow(database, id, key, sources[id]); err != nil {
			t.Fatalf("seed %s (%s): %v", id, key, err)
		}
	}
	// The collision target: another session already owns the bare form of
	// id-collision's key. Rewriting would violate idx_sessions_session_key.
	if err := seedSessionRow(database, "id-target", "11111111-2222-4333-8444-555555555555", "pi"); err != nil {
		t.Fatalf("seed collision target: %v", err)
	}

	// A turn attached to the interactive session must survive the rewrite
	// (session_turns references sessions.id, which the migration never touches).
	if _, err := database.Exec(`
		INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
		VALUES ('turn-1', 'id-interactive', '[{"role":"assistant","content":"hi"}]', 0, 'pending')`); err != nil {
		t.Fatalf("seed turn: %v", err)
	}

	// Roll goose back to v13 and re-apply 00014 (it is the last migration, so
	// no later schema needs undoing).
	rollback00014 := func() {
		t.Helper()
		if _, err := database.Exec(`DELETE FROM goose_db_version WHERE version_id >= 14`); err != nil {
			t.Fatalf("roll back goose version: %v", err)
		}
		if err := migrate(database); err != nil {
			t.Fatalf("re-migrate 00014: %v", err)
		}
	}
	rollback00014()

	// sess_<uuid> interactive row rewritten to the bare form the parser now
	// produces — future uploads merge into the same row.
	if got := sessionKey(t, database, "id-interactive"); got != "f9c1b16f-b6d1-4bd3-adb0-d0212af43158" {
		t.Fatalf("interactive key = %q, want f9c1b16f-b6d1-4bd3-adb0-d0212af43158", got)
	}
	// Non-reducible subagent id left alone (old session stays reachable).
	if got := sessionKey(t, database, "id-subagent"); got != "sess_subagent_agent_a6f1b071-e073-411c-81d4-04f787791151" {
		t.Fatalf("subagent key = %q, want unchanged", got)
	}
	// Already-bare key untouched.
	if got := sessionKey(t, database, "id-alreadybare"); got != "01a0485f-e20b-7fbc-a2e3-36abe88ce1e8" {
		t.Fatalf("bare key = %q, want unchanged", got)
	}
	// Other sources untouched even with a reducible key.
	if got := sessionKey(t, database, "id-cursor"); got != "sess_1682106d-8aae-404d-aca6-e6581a52b1fb" {
		t.Fatalf("cursor key = %q, want unchanged", got)
	}
	// Collision guard: target key exists, so the sess_ row is left alone.
	if got := sessionKey(t, database, "id-collision"); got != "sess_11111111-2222-4333-8444-555555555555" {
		t.Fatalf("collision key = %q, want unchanged", got)
	}
	// The untouched bare target is still there.
	if got := sessionKey(t, database, "id-target"); got != "11111111-2222-4333-8444-555555555555" {
		t.Fatalf("target key = %q, want unchanged", got)
	}
	// The existing turn is still attached to the same session row.
	if got := count(t, database, `SELECT COUNT(*) FROM session_turns WHERE session_id = 'id-interactive'`); got != 1 {
		t.Fatalf("turns on rewritten session = %d, want 1", got)
	}

	// Idempotency: re-apply 00014 on already-normalized data — a no-op that
	// must not double-strip, collide, or otherwise change any key.
	rollback00014()
	want := map[string]string{
		"id-interactive": "f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
		"id-subagent":    "sess_subagent_agent_a6f1b071-e073-411c-81d4-04f787791151",
		"id-alreadybare": "01a0485f-e20b-7fbc-a2e3-36abe88ce1e8",
		"id-cursor":      "sess_1682106d-8aae-404d-aca6-e6581a52b1fb",
		"id-collision":   "sess_11111111-2222-4333-8444-555555555555",
		"id-target":      "11111111-2222-4333-8444-555555555555",
	}
	for id, key := range want {
		if got := sessionKey(t, database, id); got != key {
			t.Fatalf("after re-run: %s key = %q, want %q", id, got, key)
		}
	}
}

func seedSessionRow(database *sql.DB, id, sessionKey, source string) error {
	_, err := database.Exec(`
		INSERT INTO sessions (id, session_key, source, created_at, updated_at)
		VALUES (?, ?, ?, 0, 0)`,
		id, sessionKey, source)
	return err
}

func sessionKey(t *testing.T, database *sql.DB, id string) string {
	t.Helper()
	var key string
	if err := database.QueryRow(`SELECT session_key FROM sessions WHERE id = ?`, id).Scan(&key); err != nil {
		t.Fatalf("session_key for %s: %v", id, err)
	}
	return key
}
