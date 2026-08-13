package recall_test

import (
	"context"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/colinleefish/rmb-desktop/internal/recall"
)

func TestVectorMemories_KNN(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	insert := func(id, uri, body string, vec []float32) {
		blob, err := sqlite_vec.SerializeFloat32(vec)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, embedding, created_at, updated_at)
			VALUES (?, ?, 'entities', 1, ?, ?, '[]', '[]', ?, ?, ?)`,
			id, uri, body, body, blob, nowMS, nowMS); err != nil {
			t.Fatal(err)
		}
	}

	insert("m-a", "rmb://entities/a", "x-axis", []float32{1, 0, 0, 0})
	insert("m-b", "rmb://entities/b", "y-axis", []float32{0, 1, 0, 0})
	insert("m-c", "rmb://entities/c", "diagonal", []float32{0.7071, 0.7071, 0, 0})

	// Query close to the x-axis vector: expect m-a first, m-c second, m-b last.
	matches, err := recall.VectorMemories(context.Background(), database, []float32{1, 0, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	if matches[0].URI != "rmb://entities/a" {
		t.Fatalf("expected exact match first, got %s", matches[0].URI)
	}
	if matches[1].URI != "rmb://entities/c" {
		t.Fatalf("expected diagonal second, got %s", matches[1].URI)
	}
	if matches[2].URI != "rmb://entities/b" {
		t.Fatalf("expected orthogonal last, got %s", matches[2].URI)
	}
	// Rank is similarity (1 = identical).
	if matches[0].Rank < 0.999 {
		t.Fatalf("expected near-1 similarity for exact match, got %v", matches[0].Rank)
	}
}

func TestVectorMemories_SkipsNilAndSuperseded(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	insert := func(id, uri string, vec []float32, superseded *int64) {
		blob, err := sqlite_vec.SerializeFloat32(vec)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, embedding, superseded_at, created_at, updated_at)
			VALUES (?, ?, 'entities', 1, ?, ?, '[]', '[]', ?, ?, ?, ?)`,
			id, uri, uri, uri, blob, superseded, nowMS, nowMS); err != nil {
			t.Fatal(err)
		}
	}

	supersededAt := nowMS
	insert("m-live", "rmb://entities/live", []float32{1, 0, 0, 0}, nil)
	insert("m-old", "rmb://entities/old", []float32{1, 0, 0, 0}, &supersededAt)
	// A row with no embedding.
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, embedding, created_at, updated_at)
		VALUES ('m-novec', 'rmb://entities/novec', 'entities', 1, 'no', 'no', '[]', '[]', NULL, ?, ?)`,
		nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	matches, err := recall.VectorMemories(context.Background(), database, []float32{1, 0, 0, 0}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected only the live memory, got %d: %+v", len(matches), matches)
	}
	if matches[0].URI != "rmb://entities/live" {
		t.Fatalf("expected live memory, got %s", matches[0].URI)
	}
}
