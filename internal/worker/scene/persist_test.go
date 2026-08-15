package scene

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
	"github.com/google/uuid"
)

// openDB opens a fully migrated SQLite database in a per-test temp dir.
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func mustExec(t *testing.T, d *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := d.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func sceneCount(t *testing.T, d *sql.DB, sessionID string) int {
	t.Helper()
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM scenes WHERE session_id = ?`, sessionID).Scan(&n); err != nil {
		t.Fatalf("count scenes: %v", err)
	}
	return n
}

// ftsRowIDMatches returns true if scenes_fts MATCH query resolves to the given
// scene rowid. Used to confirm the FTS index actually reflects persisted content.
func ftsMatches(t *testing.T, d *sql.DB, term string) []int64 {
	t.Helper()
	rows, err := d.Query(`SELECT rowid FROM scenes_fts WHERE scenes_fts MATCH ?`, term)
	if err != nil {
		t.Fatalf("fts match %q: %v", term, err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var r int64
		if err := rows.Scan(&r); err != nil {
			t.Fatalf("scan fts row: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// newWorker builds a scene.Worker wired to a migrated DB with a stub LLM.
// stubSceneBuilder satisfies the SceneBuilder interface. persistScenes does not
// invoke the LLM, so these methods are never called in these tests.
type stubSceneBuilder struct{}

func (stubSceneBuilder) BuildScenes(context.Context, string) (string, error) {
	return `{"scenes":[]}`, nil
}
func (stubSceneBuilder) SummarizeSessionAbstract(context.Context, string) (string, error) {
	return "", nil
}

func newWorker(t *testing.T, d *sql.DB) *Worker {
	t.Helper()
	cfg, err := config.Default()
	if err != nil {
		t.Fatalf("config default: %v", err)
	}
	// log and reg are unused by persistScenes; pass nil (NewWorker defaults log).
	return NewWorker(d, stubSceneBuilder{}, cfg.Pipeline, workerlock.NewSessionLocks(), nil, nil)
}

// TestPersistScenes_FirstCreateDoesNotCorruptFTS is the regression test for the
// "database disk image is malformed" failure. On FTS5 external-content tables,
// issuing the 'delete' command for a rowid that exists in the content table but
// was never indexed returns SQLITE_CORRUPT. Previously persistScenes always ran
// delete-before-insert, so the very first scene for a session aborted the whole
// transaction and no scene was ever written. This test asserts that a brand-new
// scene persists successfully and is searchable.
func TestPersistScenes_FirstCreateDoesNotCorruptFTS(t *testing.T) {
	d := openDB(t)
	w := newWorker(t, d)

	sessionID := uuid.NewString()
	nowMS := time.Now().UTC().UnixMilli()
	mustExec(t, d, `INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES (?, 'sess-key', ?, ?)`, sessionID, nowMS, nowMS)
	mustExec(t, d, `INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, warmup_threshold, updated_at) VALUES (?, 'idle', 'pending', 'idle', 0, ?)`, sessionID, nowMS)

	batch := &sceneBatch{SessionKey: "sess-key", SessionID: sessionID}
	scenes := []ParsedScene{{
		DisplayName: "Identity",
		Abstract:    "colin alpha",
		Body:        "the user is colin",
		SourceAtoms: []string{"rmb://atoms/x"},
	}}

	ctx := context.Background()
	if err := w.persistScenes(ctx, batch, scenes, "session abstract"); err != nil {
		t.Fatalf("persistScenes first create: %v", err)
	}

	if got := sceneCount(t, d, sessionID); got != 1 {
		t.Fatalf("expected 1 scene after first create, got %d", got)
	}
	if len(ftsMatches(t, d, "colin")) != 1 {
		t.Fatalf("expected new scene to be FTS-searchable on first create")
	}

	// pipeline_state should have advanced to idle.
	var l2 string
	if err := d.QueryRow(`SELECT l2_status FROM pipeline_state WHERE session_id = ?`, sessionID).Scan(&l2); err != nil {
		t.Fatalf("load l2_status: %v", err)
	}
	if l2 != "idle" {
		t.Fatalf("expected l2_status=idle after persist, got %q", l2)
	}
}

// TestPersistScenes_UpdateReplacesOldFTSTerms verifies the update path: a second
// persist with new abstract/body must remove the OLD terms from scenes_fts, not
// leave them stale. This guards the delete-before-upsert ordering.
func TestPersistScenes_UpdateReplacesOldFTSTerms(t *testing.T) {
	d := openDB(t)
	w := newWorker(t, d)

	sessionID := uuid.NewString()
	nowMS := time.Now().UTC().UnixMilli()
	mustExec(t, d, `INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES (?, 'sess-key', ?, ?)`, sessionID, nowMS, nowMS)
	mustExec(t, d, `INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, warmup_threshold, updated_at) VALUES (?, 'idle', 'pending', 'idle', 0, ?)`, sessionID, nowMS)

	batch := &sceneBatch{SessionKey: "sess-key", SessionID: sessionID}
	ctx := context.Background()

	// First create with term "alpha".
	if err := w.persistScenes(ctx, batch, []ParsedScene{{
		DisplayName: "Identity", Abstract: "alpha", Body: "alpha body", SourceAtoms: []string{"rmb://atoms/a"},
	}}, "abs"); err != nil {
		t.Fatalf("persistScenes create: %v", err)
	}
	if len(ftsMatches(t, d, "alpha")) != 1 {
		t.Fatalf("expected 'alpha' indexed after create")
	}

	// Reset to pending so persistScenes runs again (it sets idle on success).
	mustExec(t, d, `UPDATE pipeline_state SET l2_status='pending' WHERE session_id=?`, sessionID)

	// Update with term "bravo" replacing "alpha".
	if err := w.persistScenes(ctx, batch, []ParsedScene{{
		DisplayName: "Identity", Abstract: "bravo", Body: "bravo body", SourceAtoms: []string{"rmb://atoms/a"},
	}}, "abs"); err != nil {
		t.Fatalf("persistScenes update: %v", err)
	}

	if got := sceneCount(t, d, sessionID); got != 1 {
		t.Fatalf("expected 1 scene after update, got %d", got)
	}
	if len(ftsMatches(t, d, "bravo")) != 1 {
		t.Fatalf("expected 'bravo' indexed after update")
	}
	if len(ftsMatches(t, d, "alpha")) != 0 {
		t.Fatalf("stale FTS term 'alpha' still present after update; delete-before-upsert ordering is wrong")
	}
}

// TestPersistScenes_PruneRemovedScenes confirms that scenes absent from a later
// persist batch are pruned, and their FTS entries disappear.
func TestPersistScenes_PruneRemovedScenes(t *testing.T) {
	d := openDB(t)
	w := newWorker(t, d)

	sessionID := uuid.NewString()
	nowMS := time.Now().UTC().UnixMilli()
	mustExec(t, d, `INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES (?, 'sess-key', ?, ?)`, sessionID, nowMS, nowMS)
	mustExec(t, d, `INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, warmup_threshold, updated_at) VALUES (?, 'idle', 'pending', 'idle', 0, ?)`, sessionID, nowMS)

	batch := &sceneBatch{SessionKey: "sess-key", SessionID: sessionID}
	ctx := context.Background()

	if err := w.persistScenes(ctx, batch, []ParsedScene{
		{DisplayName: "Keep", Abstract: "keepterm", Body: "k", SourceAtoms: []string{"rmb://atoms/k"}},
		{DisplayName: "Gone", Abstract: "goneterm", Body: "g", SourceAtoms: []string{"rmb://atoms/g"}},
	}, "abs"); err != nil {
		t.Fatalf("persistScenes create two: %v", err)
	}
	if got := sceneCount(t, d, sessionID); got != 2 {
		t.Fatalf("expected 2 scenes, got %d", got)
	}

	mustExec(t, d, `UPDATE pipeline_state SET l2_status='pending' WHERE session_id=?`, sessionID)

	// Second persist drops "Gone", keeps "Keep".
	if err := w.persistScenes(ctx, batch, []ParsedScene{
		{DisplayName: "Keep", Abstract: "keepterm", Body: "k", SourceAtoms: []string{"rmb://atoms/k"}},
	}, "abs"); err != nil {
		t.Fatalf("persistScenes prune: %v", err)
	}
	if got := sceneCount(t, d, sessionID); got != 1 {
		t.Fatalf("expected 1 scene after prune, got %d", got)
	}
	if len(ftsMatches(t, d, "goneterm")) != 0 {
		t.Fatalf("pruned scene FTS term 'goneterm' still present")
	}
	if len(ftsMatches(t, d, "keepterm")) != 1 {
		t.Fatalf("kept scene FTS term 'keepterm' missing")
	}
}
