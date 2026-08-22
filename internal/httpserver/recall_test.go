package httpserver_test

import (
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

func TestSearch_sinceUntil_filtering(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	oldMS := nowMS - 40*24*time.Hour.Milliseconds()

	insert := func(id, uri string, ts int64) {
		t.Helper()
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
			VALUES (?, ?, 'entities', 1, 'k8s', 'kubectl apply deployment yaml', '[]', '[]', ?, ?)`,
			id, uri, ts, ts); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories_fts(rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), 'k8s', 'kubectl apply deployment yaml')`, id); err != nil {
			t.Fatal(err)
		}
	}
	insert("44444444-4444-4444-8444-444444444444", "rmb://entities/new", nowMS)
	insert("55555555-5555-4555-8555-555555555555", "rmb://entities/old", oldMS)

	cfg, _ := config.Default()
	srv := httpserver.New(database, cfg, filepath.Join(t.TempDir(), "config.yaml"), nil, nil, nil)

	get := func(url string) []string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", url, rec.Code, rec.Body.String())
		}
		var out struct {
			Items []struct {
				URI string `json:"uri"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		var uris []string
		for _, it := range out.Items {
			uris = append(uris, it.URI)
		}
		return uris
	}

	// Default scope includes memory + scene + skill; constrain to memory for
	// determinism.
	base := "/api/v1/search?q=kubectl%20deployment&scope=memory"

	if uris := get(base); len(uris) != 2 {
		t.Fatalf("unfiltered: want 2, got %v", uris)
	}
	if uris := get(base + "&since=7d"); len(uris) != 1 || uris[0] != "rmb://entities/new" {
		t.Fatalf("since=7d: got %v", uris)
	}
	if uris := get(base + "&until=2026-08-02"); len(uris) != 1 || uris[0] != "rmb://entities/old" {
		t.Fatalf("until: got %v", uris)
	}
	// Bad since value → 400.
	req := httptest.NewRequest(http.MethodGet, base+"&since=yesterday", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad since: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
