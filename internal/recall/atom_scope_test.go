package recall_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/colinleefish/rmb-desktop/internal/recall"
)

// insertAtomFixture inserts a session, an atom (content + FTS + embedding) and
// an owning scene referencing the atom, so both recall legs and the drill-down
// annotation can be exercised.
func insertAtomFixture(t *testing.T, database *sql.DB, atomContent string) {
	t.Helper()
	const (
		sessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		atomID    = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
		sceneID   = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	)
	nowMS := time.Now().UTC().UnixMilli()
	if _, err := database.Exec(`
		INSERT INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES (?, 'fixture-session', '', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
		VALUES (?, ?, 'openresty', '', '', '["`+atomID+`"]', NULL, ?, ?)`,
		sceneID, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	blob, err := sqlite_vec.SerializeFloat32([]float32{1, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO atoms (id, session_id, category, priority, scene_name, slug, content, source_turn_ids, embedding, created_at, updated_at)
		VALUES (?, ?, 'entities', 50, 'openresty', 'openresty-resolver', ?, '[]', ?, ?, ?)`,
		atomID, sessionID, atomContent, blob, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO atoms_fts(rowid, content)
		VALUES ((SELECT rowid FROM atoms WHERE id = ?), ?)`, atomID, atomContent); err != nil {
		t.Fatal(err)
	}
}

func TestSearchAtomScope_FTS(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	insertAtomFixture(t, database, "openresty request time DNS resolver 10.0.0.1 10.0.0.2 upstreams.conf")

	svc := recall.NewService(database)
	matches, err := svc.Search(context.Background(), nil, "openresty resolver", 5, []string{"atom"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected atom hit for atom scope")
	}
	if matches[0].Tier != "atoms" {
		t.Fatalf("expected tier atoms, got %s", matches[0].Tier)
	}
	if !strings.HasPrefix(matches[0].URI, "rmb://atoms/") {
		t.Fatalf("expected atoms uri, got %s", matches[0].URI)
	}
	// Drill-down annotation: owning scene + session.
	if !strings.Contains(matches[0].Snippet, "scene: rmb://scenes/") ||
		!strings.Contains(matches[0].Snippet, "session: rmb://sessions/") {
		t.Fatalf("expected scene+session annotation in snippet, got %q", matches[0].Snippet)
	}
}

func TestSearchAtomScope_Vector(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	insertAtomFixture(t, database, "proxy resolver details behind the gateway")

	svc := recall.NewService(database)
	embed := recall.QueryEmbedder(func(_ context.Context, q string) ([]float32, error) {
		return []float32{1, 0, 0, 0}, nil // matches the atom embedding exactly
	})
	matches, err := svc.Search(context.Background(), embed, "proxy", 5, []string{"atom"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected vector atom hit")
	}
	if matches[0].URI != "rmb://atoms/bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" {
		t.Fatalf("expected exact atom match first, got %s", matches[0].URI)
	}
}

func TestSearchAtomScope_NotDefault(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	insertAtomFixture(t, database, "kubectl deployment command yaml apply")

	svc := recall.NewService(database)
	// Default scope (memory + skill) must NOT surface atoms.
	matches, err := svc.Search(context.Background(), nil, "kubectl deployment", 5, nil, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range matches {
		if m.Tier == "atoms" {
			t.Fatal("atoms leaked into default scope")
		}
	}
}

func TestSearchAtomScope_TimeWindow(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	atomContent := "request time DNS resolver 10.0.0.1 upstreams.conf"
	insertAtomFixture(t, database, atomContent)

	// Age the atom so a --since window excludes it.
	if _, err := database.Exec(`UPDATE atoms SET updated_at = 1000000000000`); err != nil {
		t.Fatal(err)
	}

	svc := recall.NewService(database)
	matches, err := svc.Search(context.Background(), nil, atomContent, 5,
		[]string{"atom"}, recall.TimeWindow{SinceMS: time.Now().UTC().Add(-time.Hour).UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no atom hits outside time window, got %d", len(matches))
	}
}

func TestSearchAtomScope_InvalidScopeRejected(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	svc := recall.NewService(database)
	if _, err := svc.Search(context.Background(), nil, "anything", 5, []string{"atom", "bogus"}, recall.TimeWindow{}); err == nil {
		t.Fatal("expected error for invalid scope")
	}
}
