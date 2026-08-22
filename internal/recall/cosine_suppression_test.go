package recall_test

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/colinleefish/rmb-desktop/internal/recall"
)

// insertMemoryForCos inserts an active memory with an FTS row and an explicit
// embedding blob (nil vec = no embedding). source_scene_uris is left empty so
// link-based suppression never fires — these tests exercise the cosine
// fallback in isolation.
func insertMemoryForCos(t *testing.T, database *sql.DB, id, uri, abstract, body string, vec []float32) {
	t.Helper()
	nowMS := time.Now().UTC().UnixMilli()
	var blob any
	if vec != nil {
		b, err := sqlite_vec.SerializeFloat32(vec)
		if err != nil {
			t.Fatal(err)
		}
		blob = b
	}
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, embedding, created_at, updated_at)
		VALUES (?, ?, 'events', 1, ?, ?, '[]', '[]', ?, ?, ?)`,
		id, uri, abstract, body, blob, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`, id, abstract, body); err != nil {
		t.Fatal(err)
	}
}

// insertSceneForCos inserts a scene with an FTS row and an explicit embedding
// blob (nil vec = no embedding).
func insertSceneForCos(t *testing.T, database *sql.DB, id, abstract, body string, vec []float32) {
	t.Helper()
	nowMS := time.Now().UTC().UnixMilli()
	var blob any
	if vec != nil {
		b, err := sqlite_vec.SerializeFloat32(vec)
		if err != nil {
			t.Fatal(err)
		}
		blob = b
	}
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
		VALUES (?, 'dddddddd-dddd-4ddd-8ddd-dddddddddddd', 'dup', ?, ?, '[]', ?, ?, ?)`,
		id, abstract, body, blob, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM scenes WHERE id = ?), ?, ?)`, id, abstract, body); err != nil {
		t.Fatal(err)
	}
}

const (
	cosMemURI   = "rmb://events/2026-08-21-cluster-admin-toolbox-removal"
	cosMemID    = "11111111-1111-4111-8111-111111111111"
	cosSceneID  = "22222222-2222-4222-8222-222222222222"
	cosSceneURI = "rmb://scenes/22222222-2222-4222-8222-222222222222"
)

// TestSearch_cosineSuppression_identicalEmbedding: a scene whose stored
// embedding is identical to a co-ranked memory (byte-identical text — the
// audit's cluster-admin-toolbox pattern, but without source_scene_uris
// provenance) is suppressed by the cosine fallback and the memory is
// annotated with the (+scene dup: …) drill-down pointer.
func TestSearch_cosineSuppression_identicalEmbedding(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	vec := []float32{1, 0, 0, 0}
	insertMemoryForCos(t, database, cosMemID, cosMemURI,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", vec)
	insertSceneForCos(t, database, cosSceneID,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", vec)

	svc := recall.NewService(database)
	m, err := svc.Search(context.Background(), nil, "cluster admin toolbox removed rejected",
		10, []string{"memory", "scene"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	var sawScene, sawMem, sawDupAnno bool
	for _, hit := range m {
		if hit.URI == cosSceneURI {
			sawScene = true
		}
		if hit.URI == cosMemURI {
			sawMem = true
			if strings.Contains(hit.Snippet, "(+scene dup: "+cosSceneURI+")") {
				sawDupAnno = true
			}
		}
	}
	if sawScene {
		t.Fatalf("semantically identical scene was not suppressed: %+v", m)
	}
	if !sawMem || !sawDupAnno {
		t.Fatalf("memory missing or not annotated with scene dup: %+v", m)
	}
}

// TestSearch_cosineSuppression_distinctEmbeddingsKept: unrelated embeddings
// (cos ≈ 0) co-rank without suppression and pick up no annotation.
func TestSearch_cosineSuppression_distinctEmbeddingsKept(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	insertMemoryForCos(t, database, cosMemID, cosMemURI,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", []float32{1, 0, 0, 0})
	insertSceneForCos(t, database, cosSceneID,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", []float32{0, 1, 0, 0})

	svc := recall.NewService(database)
	m, err := svc.Search(context.Background(), nil, "cluster admin toolbox removed rejected",
		10, []string{"memory", "scene"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	var sawScene, sawMem bool
	for _, hit := range m {
		if strings.Contains(hit.Snippet, "+scene dup:") {
			t.Fatalf("unexpected dup annotation: %+v", m)
		}
		if hit.URI == cosSceneURI {
			sawScene = true
		}
		if hit.URI == cosMemURI {
			sawMem = true
		}
	}
	if !sawScene || !sawMem {
		t.Fatalf("expected both distinct hits to survive: %+v", m)
	}
}

// TestSearch_cosineSuppression_thresholdBoundary: the calibrated 0.98
// threshold separates near-identical (suppressed) from related-but-distinct
// (kept) pairs. The audit's live-store calibration: identical text ≈1.0,
// strongest random negative 0.9526, related pairs 0.78–0.90.
func TestSearch_cosineSuppression_thresholdBoundary(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	memVec := []float32{1, 0}
	hiScene := "33333333-3333-4333-8333-333333333333" // cos ≈ 0.99 → suppressed
	loScene := "44444444-4444-4444-8444-444444444444" // cos ≈ 0.97 → kept
	unit := func(c float64) []float32 {
		return []float32{float32(c), float32(math.Sqrt(1 - c*c))}
	}
	insertMemoryForCos(t, database, cosMemID, cosMemURI,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", memVec)
	insertSceneForCos(t, database, hiScene,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", unit(0.99))
	insertSceneForCos(t, database, loScene,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", unit(0.97))

	svc := recall.NewService(database)
	m, err := svc.Search(context.Background(), nil, "cluster admin toolbox removed rejected",
		10, []string{"memory", "scene"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	sawHi, sawLo := false, false
	for _, hit := range m {
		switch hit.URI {
		case "rmb://scenes/" + hiScene:
			sawHi = true
		case "rmb://scenes/" + loScene:
			sawLo = true
		}
	}
	if sawHi {
		t.Fatalf("cos 0.99 pair should be suppressed (threshold 0.98): %+v", m)
	}
	if !sawLo {
		t.Fatalf("cos 0.97 pair should survive (threshold 0.98): %+v", m)
	}
}

// TestSearch_cosineSuppression_otherTiersUntouched: atoms and skills with an
// embedding identical to a co-ranked memory are never suppressed — the
// fallback is strictly a memory×scene pass.
func TestSearch_cosineSuppression_otherTiersUntouched(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	vec := []float32{1, 0, 0, 0}
	insertMemoryForCos(t, database, cosMemID, cosMemURI,
		"openresty", "openresty request time dns resolver", vec)

	// Atom with the identical embedding and matching text.
	nowMS := time.Now().UTC().UnixMilli()
	atomID := "55555555-5555-4555-8555-555555555555"
	if _, err := database.Exec(`
		INSERT INTO atoms (id, session_id, category, priority, scene_name, slug, content, source_turn_ids, embedding, created_at, updated_at)
		VALUES (?, 'dddddddd-dddd-4ddd-8ddd-dddddddddddd', 'entities', 50, 'openresty', 'openresty-resolver', 'openresty request time dns resolver', '[]', ?, ?, ?)`,
		atomID, mustSerializeVec(t, vec), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO atoms_fts(rowid, content)
		VALUES ((SELECT rowid FROM atoms WHERE id = ?), 'openresty request time dns resolver')`, atomID); err != nil {
		t.Fatal(err)
	}

	// Skill with the identical embedding and matching description.
	if _, err := database.Exec(`
		INSERT INTO skills (id, slug, uri, version, name, description, tags, bundle_sha256, fts_text, embedding, created_at, updated_at)
		VALUES ('66666666-6666-4666-8666-666666666666', 'openresty-dns', 'rmb://skills/openresty-dns', 1, 'openresty', 'openresty request time dns resolver', '[]', 'sha', 'openresty request time dns resolver', ?, ?, ?)`,
		mustSerializeVec(t, vec), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO skills_fts(rowid, fts_text)
		VALUES ((SELECT rowid FROM skills WHERE slug = 'openresty-dns'), 'openresty request time dns resolver')`); err != nil {
		t.Fatal(err)
	}

	svc := recall.NewService(database)
	m, err := svc.Search(context.Background(), nil, "openresty request time dns resolver",
		10, []string{"memory", "atom", "skill"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	sawAtom, sawSkill := false, false
	for _, hit := range m {
		if hit.URI == "rmb://atoms/"+atomID {
			sawAtom = true
		}
		if hit.URI == "rmb://skills/openresty-dns" {
			sawSkill = true
		}
		if strings.Contains(hit.Snippet, "+scene dup:") {
			t.Fatalf("cosine pass leaked into non-scene tiers: %+v", m)
		}
	}
	if !sawAtom || !sawSkill {
		t.Fatalf("atom/skill with identical embedding must survive: %+v", m)
	}
}

// TestSearch_cosineSuppression_missingEmbeddingKept: a scene with no stored
// embedding cannot be compared, so nothing is suppressed (conservative).
func TestSearch_cosineSuppression_missingEmbeddingKept(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	insertMemoryForCos(t, database, cosMemID, cosMemURI,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", []float32{1, 0, 0, 0})
	insertSceneForCos(t, database, cosSceneID,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", nil)

	svc := recall.NewService(database)
	m, err := svc.Search(context.Background(), nil, "cluster admin toolbox removed rejected",
		10, []string{"memory", "scene"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	var sawScene bool
	for _, hit := range m {
		if hit.URI == cosSceneURI {
			sawScene = true
		}
	}
	if !sawScene {
		t.Fatalf("scene without embedding must not be suppressed: %+v", m)
	}
}

// TestSearch_cosineSuppression_sceneRankIrrelevant: even when the scene FTS
// match outranks the memory, the scene is the suppressed tier — the memory is
// the index, the scene is evidence (mirrors the link pass).
func TestSearch_cosineSuppression_sceneRankIrrelevant(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	vec := []float32{1, 0, 0, 0}
	// Scene body is a tighter bm25 match for the query, so it ranks first.
	insertMemoryForCos(t, database, cosMemID, cosMemURI,
		"cluster-admin-toolbox verbose context filler words", "cluster admin toolbox removed rejected with lots of extra words in body", vec)
	insertSceneForCos(t, database, cosSceneID,
		"cluster-admin-toolbox", "cluster admin toolbox removed rejected", vec)

	svc := recall.NewService(database)
	m, err := svc.Search(context.Background(), nil, "cluster admin toolbox removed rejected",
		10, []string{"memory", "scene"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range m {
		if hit.URI == cosSceneURI {
			t.Fatalf("scene must lose to the near-duplicate memory regardless of rank: %+v", m)
		}
	}
	if len(m) == 0 {
		t.Fatal("expected the memory to survive")
	}
}

func mustSerializeVec(t *testing.T, vec []float32) []byte {
	t.Helper()
	blob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		t.Fatal(err)
	}
	return blob
}
