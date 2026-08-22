package recall_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/recall"
)

func TestSearch_FTS_memories(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	memID := "11111111-1111-4111-8111-111111111111"
	_, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
		VALUES (?, 'rmb://entities/kubernetes', 'entities', 1, 'k8s', 'kubectl apply deployment yaml', '[]', '[]', ?, ?)`,
		memID, nowMS, nowMS)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), 'k8s', 'kubectl apply deployment yaml')`, memID)
	if err != nil {
		t.Fatal(err)
	}

	svc := recall.NewService(database)
	matches, err := svc.Search(context.Background(), nil, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected fts memory hit")
	}
	if matches[0].URI != "rmb://entities/kubernetes" {
		t.Fatalf("uri=%s", matches[0].URI)
	}
}

func TestEscapeFTSQuery_multilingual(t *testing.T) {
	q := recall.EscapeFTSQuery("李广慧 Kubernetes")
	if q == "" {
		t.Fatal("empty fts query")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rmb.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func TestSearch_defaultScope_excludesScenes(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	// memory hit
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
		VALUES ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', 'rmb://events/2026-08-21-openresty-dynamic-dns', 'events', 1, 'openresty', 'openresty dynamic dns resolver', '["rmb://scenes/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"]', '[]', ?, ?)`,
		nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'), 'openresty', 'openresty dynamic dns resolver')`); err != nil {
		t.Fatal(err)
	}
	// scene with identical text (the evidence)
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, abstract, body, created_at, updated_at)
		VALUES ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', 'openresty', 'openresty dynamic dns resolver', ?, ?)`, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM scenes WHERE id = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'), 'openresty', 'openresty dynamic dns resolver')`); err != nil {
		t.Fatal(err)
	}

	svc := recall.NewService(database)
	// Default scope: memory + skill only — no scenes.
	m, err := svc.Search(context.Background(), nil, "openresty dynamic dns", 10, nil, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range m {
		if hit.Tier == "scene" {
			t.Fatalf("scene leaked into default scope: %s", hit.URI)
		}
	}
	if len(m) != 1 || m[0].URI != "rmb://events/2026-08-21-openresty-dynamic-dns" {
		t.Fatalf("default scope hits: %+v", m)
	}
}

func TestSearch_linkSuppression_dedupesScene(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	memURI := "rmb://events/2026-08-21-openresty-dynamic-dns"
	sceneURI := "rmb://scenes/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
		VALUES ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', ?, 'events', 1, 'openresty', 'openresty dynamic dns resolver', ?, '[]', ?, ?)`,
		memURI, `["rmb://scenes/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"]`, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'), 'openresty', 'openresty dynamic dns resolver')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, abstract, body, created_at, updated_at)
		VALUES ('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', 'openresty', 'openresty dynamic dns resolver', ?, ?)`, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM scenes WHERE id = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'), 'openresty', 'openresty dynamic dns resolver')`); err != nil {
		t.Fatal(err)
	}

	svc := recall.NewService(database)
	// Explicit multi-scope search: the linked scene is suppressed, the owning
	// memory is annotated with the drill-down path.
	m, err := svc.Search(context.Background(), nil, "openresty dynamic dns", 10, []string{"memory", "scene"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	var sawScene, sawAnno bool
	for _, hit := range m {
		if hit.URI == sceneURI {
			sawScene = true
		}
		if hit.URI == memURI && hit.Snippet != "" {
			sawAnno = true
		}
	}
	if sawScene {
		t.Fatal("linked scene was not suppressed")
	}
	if !sawAnno {
		t.Fatalf("memory not annotated with scene depth: %+v", m)
	}
}

func TestSearch_skillCap_singleSlot(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	for i, slug := range []string{"aaa", "bbb", "ccc"} {
		if _, err := database.Exec(`
			INSERT INTO skills (id, slug, uri, version, name, description, tags, bundle_sha256, fts_text, created_at, updated_at)
			VALUES (?, ?, ?, 1, ?, ?, '[]', 'sha', ?, ?, ?)`,
			fmt.Sprintf("%d", i), slug, "rmb://skills/"+slug, "skill-"+slug, "generic ops english description of "+slug, "generic ops english description of "+slug, nowMS, nowMS); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO skills_fts(rowid, fts_text)
			VALUES ((SELECT rowid FROM skills WHERE slug = ?), ?)`, slug, "skill-"+slug+" generic ops english description of "+slug); err != nil {
			t.Fatal(err)
		}
	}

	svc := recall.NewService(database)
	m, err := svc.Search(context.Background(), nil, "generic ops english description", 10, nil, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	nSkill := 0
	for _, hit := range m {
		if hit.Tier == "skills" {
			nSkill++
		}
	}
	if nSkill > 1 {
		t.Fatalf("expected ≤1 skill in default scope, got %d", nSkill)
	}

	// Explicit skill-only scope lifts the cap.
	m, err = svc.Search(context.Background(), nil, "generic ops english description", 10, []string{"skill"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	nSkill = 0
	for _, hit := range m {
		if hit.Tier == "skills" {
			nSkill++
		}
	}
	if nSkill != 3 {
		t.Fatalf("skill-only scope: want 3 skills, got %d", nSkill)
	}
}
