package spike

import (
	"database/sql"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	sqlite_vec.Auto()
}

// TestSQLiteVec_cosine proves sqlite-vec loads and ranks vectors (M0 spike).
func TestSQLiteVec_cosine(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE VIRTUAL TABLE vec_items USING vec0(embedding float[4])`); err != nil {
		t.Fatal(err)
	}

	items := map[int][]float32{
		1: {0.1, 0.1, 0.1, 0.1},
		2: {0.2, 0.2, 0.2, 0.2},
		3: {0.3, 0.3, 0.3, 0.3},
	}
	for id, values := range items {
		blob, err := sqlite_vec.SerializeFloat32(values)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO vec_items(rowid, embedding) VALUES (?, ?)`, id, blob); err != nil {
			t.Fatal(err)
		}
	}

	q, err := sqlite_vec.SerializeFloat32([]float32{0.3, 0.3, 0.3, 0.3})
	if err != nil {
		t.Fatal(err)
	}

	row := db.QueryRow(`
		SELECT rowid FROM vec_items
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT 1`, q)

	var rowid int64
	if err := row.Scan(&rowid); err != nil {
		t.Fatal(err)
	}
	if rowid != 3 {
		t.Fatalf("expected rowid 3, got %d", rowid)
	}
}

// TestFTS5_unicode61_and_trigram proves bilingual FTS legs (M0 spike).
func TestFTS5_unicode61_and_trigram(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE docs_fts USING fts5(title, body, tokenize='unicode61');
		CREATE VIRTUAL TABLE docs_tri USING fts5(title, body, tokenize='trigram');
	`); err != nil {
		t.Fatal(err)
	}

	rows := []struct{ title, body string }{
		{"k8s", "kubectl apply deployment yaml"},
		{"中文", "李广慧在北京使用 Kubernetes"},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO docs_fts(title, body) VALUES (?, ?)`, r.title, r.body); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO docs_tri(title, body) VALUES (?, ?)`, r.title, r.body); err != nil {
			t.Fatal(err)
		}
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM docs_fts WHERE docs_fts MATCH 'kubectl'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unicode61 kubectl count=%d", count)
	}

	if err := db.QueryRow(`SELECT count(*) FROM docs_fts WHERE docs_fts MATCH 'Kubernetes'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unicode61 Kubernetes count=%d", count)
	}

	if err := db.QueryRow(`SELECT count(*) FROM docs_tri WHERE docs_tri MATCH 'Kube'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Fatalf("trigram substring count=%d", count)
	}
}
