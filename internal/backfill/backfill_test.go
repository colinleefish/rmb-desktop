package backfill

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/colinleefish/rmb-desktop/internal/db"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func blob(v []float32) []byte {
	b, err := sqlite_vec.SerializeFloat32(v)
	if err != nil {
		panic(err)
	}
	return b
}

// seedMemWithScene inserts one memory (empty provenance) whose embedding
// matches a seeded scene's embedding, plus an unrelated memory/scene.
func seedMemWithScene(t *testing.T, database *sql.DB) (memURI, sceneURI string) {
	t.Helper()
	nowMS := time.Now().UTC().UnixMilli()

	// Session for the scene (FK).
	sessionID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err := database.Exec(`
		INSERT OR IGNORE INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES (?, 'focused', '', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	sceneID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
		VALUES (?, ?, '', 'openresty resolver detail', 'resolver ip 10.0.0.1 upstreams.conf proc', '[]', ?, ?, ?)`,
		sceneID, sessionID, blob([]float32{1, 0, 0, 0}), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	// Unrelated scene.
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
		VALUES ('cccccccc-cccc-4ccc-8ccc-cccccccccccc', ?, '', 'unrelated', 'photos of food recipes', '[]', ?, ?, ?)`,
		sessionID, blob([]float32{0, 1, 0, 0}), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	memURI = "rmb://events/2026-08-21-openresty-resolver"
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, embedding, created_at, updated_at)
		VALUES ('dddddddd-dddd-4ddd-8ddd-dddddddddddd', ?, 'events', 1, 's', 'resolver ip 10.0.0.1 upstreams proc', '[]', '[]', ?, ?, ?)`,
		memURI, blob([]float32{1, 0.05, 0, 0}), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	return memURI, "rmb://scenes/" + sceneID
}

func TestBackfillProvenance_linksMatchingScene(t *testing.T) {
	database := openDB(t)
	memURI, sceneURI := seedMemWithScene(t, database)

	stats, err := BackfillProvenance(context.Background(), database, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.MemoriesScanned != 1 {
		t.Fatalf("scanned=%d want 1", stats.MemoriesScanned)
	}
	if stats.MemoriesLinked != 1 || stats.ScenesLinked != 1 {
		t.Fatalf("linked=%d scenes=%d want 1/1", stats.MemoriesLinked, stats.ScenesLinked)
	}

	var src string
	if err := database.QueryRow(`SELECT source_scene_uris FROM memories WHERE uri = ?`, memURI).Scan(&src); err != nil {
		t.Fatal(err)
	}
	uris, _ := db.UnmarshalStringArray(src)
	if len(uris) != 1 || uris[0] != sceneURI {
		t.Fatalf("provenance = %v want [%s]", uris, sceneURI)
	}
}

func TestBackfillProvenance_idempotentAndDryRun(t *testing.T) {
	database := openDB(t)
	memURI, _ := seedMemWithScene(t, database)

	// Dry-run: reports but does not write.
	dry, err := BackfillProvenance(context.Background(), database, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if dry.MemoriesLinked != 1 {
		t.Fatalf("dry-run linked=%d want 1", dry.MemoriesLinked)
	}
	var src string
	if err := database.QueryRow(`SELECT source_scene_uris FROM memories WHERE uri = ?`, memURI).Scan(&src); err != nil {
		t.Fatal(err)
	}
	if src != "[]" {
		t.Fatalf("dry-run must not write, got %q", src)
	}

	// Real pass.
	if _, err := BackfillProvenance(context.Background(), database, Options{}); err != nil {
		t.Fatal(err)
	}
	// Idempotency: second pass finds zero empty-provenance memories.
	stats, err := BackfillProvenance(context.Background(), database, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.MemoriesScanned != 0 {
		t.Fatalf("re-run scanned=%d want 0 (idempotent)", stats.MemoriesScanned)
	}

	// Confirm determinism of the written value.
	if err := database.QueryRow(`SELECT source_scene_uris FROM memories WHERE uri = ?`, memURI).Scan(&src); err != nil {
		t.Fatal(err)
	}
	if src == "[]" {
		t.Fatal("expected provenance written")
	}
}

func TestBackfillProvenance_skipsMemoryWithExistingProvenance(t *testing.T) {
	database := openDB(t)
	nowMS := time.Now().UTC().UnixMilli()
	sessionID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err := database.Exec(`
		INSERT OR IGNORE INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES (?, 'x', '', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	// A memory that already has provenance must be left alone (embedding
	// matches no scene here anyway — but it shouldn't even be scanned).
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, embedding, created_at, updated_at)
		VALUES ('eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee', 'rmb://events/2026-08-01-has-provenance', 'events', 1, '', 'body', '["rmb://scenes/cc"]', '[]', ?, ?, ?)`,
		blob([]float32{1, 0, 0, 0}), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	stats, err := BackfillProvenance(context.Background(), database, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.MemoriesScanned != 0 {
		t.Fatalf("scanned=%d want 0 (existing provenance skipped)", stats.MemoriesScanned)
	}
}
