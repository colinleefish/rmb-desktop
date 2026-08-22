package eval_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/colinleefish/rmb-desktop/internal/recall"
	"github.com/colinleefish/rmb-desktop/internal/recall/eval"
)

const dupAnnoPrefix = "(+scene dup: "

// TestCosineSuppression_fixtureNoFalsePositives runs every golden query with
// an explicit memory+scene scope over the pristine fixture (whose scenes all
// lack source_scene_uris, so the cosine fallback is the only dedup path) and
// asserts that any suppressed scene really is a byte-identical duplicate of
// the surviving memory. Random negatives must never be fused (issue #26
// acceptance).
func TestCosineSuppression_fixtureNoFalsePositives(t *testing.T) {
	fix, err := eval.LoadFixture("testdata/golden_fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := eval.LoadGolden("golden.yaml")
	if err != nil {
		t.Fatal(err)
	}
	database, err := fix.BuildDB(filepath.Join(t.TempDir(), "scratch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	memText := map[string]eval.MemoryRow{}
	for _, m := range fix.Memories {
		// Only active rows are visible to search (superseded_at IS NULL).
		if m.SupersededAt == nil {
			memText[m.URI] = m
		}
	}
	sceneText := map[string]eval.SceneRow{}
	for _, s := range fix.Scenes {
		sceneText["rmb://scenes/"+s.ID] = s
	}

	identical := func(a eval.MemoryRow, s eval.SceneRow) bool {
		pairs := [][2]string{
			{a.Abstract, s.Abstract}, {a.Abstract, s.Body},
			{a.Body, s.Abstract}, {a.Body, s.Body},
		}
		for _, p := range pairs {
			if p[0] != "" && p[0] == p[1] {
				return true
			}
		}
		return false
	}

	svc := recall.NewService(database)
	suppressions := 0
	for _, q := range golden.Questions {
		matches, err := svc.Search(context.Background(), nil, q.Query, 10, []string{"memory", "scene"}, recall.TimeWindow{})
		if err != nil {
			t.Fatalf("query %s: %v", q.ID, err)
		}
		for _, m := range matches {
			idx := strings.Index(m.Snippet, dupAnnoPrefix)
			if idx < 0 {
				continue
			}
			suppressions++
			uri := strings.TrimSuffix(m.Snippet[idx+len(dupAnnoPrefix):], ")")
			uri = strings.TrimSpace(strings.SplitN(uri, ")", 2)[0])
			sRow, okS := sceneText[uri]
			mRow, okM := memText[m.URI]
			if !okS || !okM {
				t.Fatalf("%s: dup annotation references unknown rows %s / %s", q.ID, uri, m.URI)
			}
			if !identical(mRow, sRow) {
				t.Fatalf("%s: suppressed scene is not a byte-identical duplicate (false positive): mem=%s scene=%s",
					q.ID, m.URI, uri)
			}
		}
	}
	t.Logf("cross-tier suppressions across %d golden queries (memory,scene k=10): %d (all verified as true duplicates)",
		len(golden.Questions), suppressions)
}

// TestCosineSuppression_fixtureTrueDuplicateFused injects a provenance-free
// scene that is an exact copy of an ACTIVE memory's text (embedding included,
// recomputed with the fixture's own hash embedder) and asserts the fallback
// suppresses the scene and annotates the memory on a real fixture-scale DB.
// This is the fallback's raison d'être: the fixture's scenes carry no
// source_scene_uris, so link-based suppression cannot fire.
func TestCosineSuppression_fixtureTrueDuplicateFused(t *testing.T) {
	fix, err := eval.LoadFixture("testdata/golden_fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	database, err := fix.BuildDB(filepath.Join(t.TempDir(), "scratch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	// Deterministic victim: first active memory with a non-empty abstract.
	var mem eval.MemoryRow
	for _, m := range fix.Memories {
		if m.SupersededAt == nil && strings.TrimSpace(m.Abstract) != "" {
			mem = m
			break
		}
	}
	if mem.URI == "" {
		t.Fatal("fixture has no active memories")
	}

	// Inject a new scene that duplicates the memory byte-for-byte. The
	// fixture embeds "\n"+abstract+"\n"+body (same join as BuildDB), so the
	// scene's stored embedding is identical to the memory's. The scenes_fts
	// triggers index the row on INSERT automatically.
	const (
		sceneID  = "99999999-9999-4999-8999-999999999999"
		sceneURI = "rmb://scenes/99999999-9999-4999-8999-999999999999"
	)
	blob, err := sqlite_vec.SerializeFloat32(eval.HashEmbed("\n" + mem.Abstract + "\n" + mem.Body))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
		VALUES (?, 'dddddddd-dddd-4ddd-8ddd-dddddddddddd', 'dup-scene', ?, ?, '[]', ?, ?, ?)`,
		sceneID, mem.Abstract, mem.Body, blob, mem.CreatedAt, mem.UpdatedAt); err != nil {
		t.Fatal(err)
	}

	// Query on the memory's own abstract tokens: FTS matches both the memory
	// and its byte-identical scene twin, with no provenance link between them.
	tokens := strings.Fields(mem.Abstract)
	if len(tokens) > 5 {
		tokens = tokens[:5]
	}
	query := strings.Join(tokens, " ")

	svc := recall.NewService(database)
	matches, err := svc.Search(context.Background(), nil, query, 10, []string{"memory", "scene"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	var sawScene, sawMem, sawAnno bool
	for _, m := range matches {
		if m.URI == sceneURI {
			sawScene = true
		}
		if m.URI == mem.URI {
			sawMem = true
			if strings.Contains(m.Snippet, dupAnnoPrefix+sceneURI+")") {
				sawAnno = true
			}
		}
	}
	if !sawMem {
		t.Fatalf("victim memory %s did not rank for %q: %+v", mem.URI, query, matches)
	}
	if sawScene {
		t.Fatalf("exact-duplicate scene not suppressed for %q: %+v", query, matches)
	}
	if !sawAnno {
		t.Fatalf("memory not annotated with its suppressed dup: %+v", matches)
	}
}
