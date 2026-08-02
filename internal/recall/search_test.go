package recall_test

import (
	"context"
	"database/sql"
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
	matches, err := svc.Search(context.Background(), nil, "kubectl deployment", 5, []string{"memory"})
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
