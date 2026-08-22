package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/httpserver"
)

// TestArchive_endToEndRoundTrip is the P3.3 acceptance test (issue #32):
//  1. cold memories are proposed by the doctor,
//  2. archiving removes them from default search but they stay cat-able,
//  3. restoring brings them back,
//  4. the default-search candidate pool measurably shrinks then recovers.
func TestArchive_endToEndRoundTrip(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	old := nowMS - 200*24*time.Hour.Milliseconds() // ~200 days cold

	// Three memories that all FTS-match the query "kubectl deployment".
	insert := func(id, uri, abs string, updatedMS int64, hot bool) {
		t.Helper()
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
			VALUES (?, ?, 'entities', 1, ?, ?, '[]', '[]', ?, ?)`,
			id, uri, abs, abs, updatedMS, updatedMS); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories_fts(rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`, id, abs, abs); err != nil {
			t.Fatal(err)
		}
		if hot {
			// A "hot" memory: qualifying use yesterday → heat signal and recent
			// last_use_at, so it is NOT an archival candidate.
			if _, err := database.Exec(`
				INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, heat, last_use_at, updated_at)
				VALUES (?, 0, 1, 0, 3.0, ?, ?)`, uri, nowMS-time.Hour.Milliseconds(), nowMS); err != nil {
				t.Fatal(err)
			}
		}
	}
	insert("aaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "rmb://entities/cold-a", "kubectl deployment yaml", old, false)
	insert("bbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "rmb://events/cold-b", "kubectl deployment yaml", old, false)
	insert("ccc-cccc-cccc-cccc-cccccccccccc", "rmb://entities/warm-c", "kubectl deployment yaml", old, true)

	cfg, _ := config.Default()
	srv := httpserver.New(database, cfg, filepath.Join(t.TempDir(), "config.yaml"), nil, nil, nil)

	search := func() []string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=kubectl%20deployment&scope=memory", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("search status=%d body=%s", rec.Code, rec.Body.String())
		}
		var out struct {
			Items []struct {
				URI string `json:"uri"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		var uris []string
		for _, it := range out.Items {
			uris = append(uris, it.URI)
		}
		return uris
	}

	candidates := func() map[string]any {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/doctor/archive", nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("candidates status=%d body=%s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	action := func(body string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/doctor/archive", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("action status=%d body=%s", rec.Code, rec.Body.String())
		}
		var out map[string]int
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		for _, v := range out {
			return v
		}
		return 0
	}

	cat := func(uri string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/inspect/cat?uri="+uri, nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			return "ERR:" + rec.Body.String()
		}
		return rec.Body.String()
	}

	// Baseline: all three in default search.
	all := search()
	if len(all) != 3 {
		t.Fatalf("baseline default search = %v, want 3 hits", all)
	}

	// Doctor proposes the two cold memories (dry-run), warm excluded.
	if c := candidates()["count"]; c != float64(2) {
		t.Fatalf("doctor proposed %v candidates, want 2", c)
	}

	// Explicit approval: bulk archive the proposed set.
	if n := action(`{"action":"archive"}`); n != 2 {
		t.Fatalf("archive applied %d, want 2", n)
	}

	// Default-search pool shrank 3 → 1 (cold rows leave default search).
	after := search()
	if len(after) != 1 || after[0] != "rmb://entities/warm-c" {
		t.Fatalf("after archive, default search = %v, want only warm-c", after)
	}

	// Archived rows remain cat-able by direct uri.
	if body := cat("rmb://entities/cold-a"); body != "kubectl deployment yaml" {
		t.Fatalf("cat of archived uri = %q, want body text", body)
	}

	// Restore cold-a only → back in default search (pool 1 → 2).
	if n := action(`{"action":"restore","uris":["rmb://entities/cold-a"]}`); n != 1 {
		t.Fatalf("restore cold-a applied %d, want 1", n)
	}
	restored := search()
	if len(restored) != 2 {
		t.Fatalf("after restore cold-a, default search = %v, want 2 hits", restored)
	}

	// Restore everything → full pool back (3).
	if n := action(`{"action":"restore","all":true}`); n != 1 {
		t.Fatalf("restore-all applied %d, want 1 (cold-b only)", n)
	}
	if all2 := search(); len(all2) != 3 {
		t.Fatalf("after restore-all, default search = %v, want 3 hits", all2)
	}
}

// TestArchive_memoriesOnly asserts scenes/atoms/skills are never archived:
// the archival endpoints touch the memories table exclusively, and cat/scene
// retrieval is unaffected.
func TestArchive_memoriesOnly(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	old := nowMS - 200*24*time.Hour.Milliseconds()

	// A cold scene (evidence tier — must never be an archive candidate or be
	// filtered).
	if _, err := database.Exec(`
		INSERT INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES ('sess-1', 'fixture-s', '', ?, ?)`, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
		VALUES ('scn-1', 'sess-1', 'openresty', 'openresty dynamic dns', '', '[]', NULL, ?, ?)`, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO scenes_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM scenes WHERE id = 'scn-1'), 'openresty dynamic dns', '')`); err != nil {
		t.Fatal(err)
	}

	cfg, _ := config.Default()
	srv := httpserver.New(database, cfg, filepath.Join(t.TempDir(), "config.yaml"), nil, nil, nil)

	// Archive-all must not touch the scene, and scene search still works.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/doctor/archive", bytes.NewBufferString(`{"action":"archive"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("archive-all status=%d", rec.Code)
	}
	var ar map[string]int
	_ = json.Unmarshal(rec.Body.Bytes(), &ar)
	if ar["archived"] != 0 {
		t.Fatalf("archive-all touched %d rows, want 0 (no cold memories yet)", ar["archived"])
	}

	sreq := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=openresty%20dynamic%20dns&scope=scene", nil)
	srec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(srec, sreq)
	if srec.Code != 200 {
		t.Fatalf("scene search status=%d", srec.Code)
	}
	if !bytes.Contains(srec.Body.Bytes(), []byte("rmb://scenes/scn-1")) {
		t.Fatalf("scene still searchable after archive: %s", srec.Body.String())
	}
}
