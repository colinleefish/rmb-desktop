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

// TestDoctorMetricsEndpoint exercises the search → query-log → cat → join →
// doctor pipeline through the HTTP layer (temp DB only).
func TestDoctorMetricsEndpoint(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	cfg, _ := config.Default()
	srv := httpserver.New(database, cfg, filepath.Join(t.TempDir(), "config.yaml"), nil, nil, nil)
	h := srv.Handler()

	do := func(method, url string, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, url, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != wantStatus {
			t.Fatalf("%s %s: status=%d body=%s", method, url, rec.Code, rec.Body.String())
		}
		return rec
	}

	// Two searches that hit an indexed memory; then cat one of them (within
	// the 10-min join window).
	insertMemory := func(id, uri string) {
		t.Helper()
		nowMS := time.Now().UTC().UnixMilli()
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
			VALUES (?, ?, 'entities', 1, 'k8s', 'kubectl apply deployment yaml', '[]', '[]', ?, ?)`,
			id, uri, nowMS, nowMS); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories_fts(rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), 'k8s', 'kubectl apply deployment yaml')`, id); err != nil {
			t.Fatal(err)
		}
	}
	insertMemory("66666666-6666-4666-8666-666666666666", "rmb://entities/heat-target")

	// Async telemetry needs a beat to flush.
	do(http.MethodGet, "/api/v1/search?q=kubectl+deployment", http.StatusOK)
	do(http.MethodGet, "/api/v1/search?q=kubectl+deployment", http.StatusOK)
	do(http.MethodGet, "/api/v1/inspect/cat?uri=rmb://entities/heat-target", http.StatusOK)
	deadline := time.Now().Add(3 * time.Second)
	for {
		var n int
		_ = database.QueryRow(`SELECT COUNT(*) FROM search_queries WHERE catted_uri IS NOT NULL`).Scan(&n)
		if n == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	rec := do(http.MethodGet, "/api/v1/doctor/metrics", http.StatusOK)
	var m struct {
		Searches    int64   `json:"searches"`
		Converted   int64   `json:"converted_searches"`
		ZeroCatRate float64 `json:"zero_cat_search_rate"`
		TotalCats   int64   `json:"total_cats"`
		HeatConc    float64 `json:"heat_concentration"`
		HeatAlarm   bool    `json:"heat_concentration_alarm"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if m.Searches < 2 || m.Converted < 1 {
		t.Fatalf("searches=%d converted=%d (want >=2/>=1)", m.Searches, m.Converted)
	}
	if m.TotalCats < 1 {
		t.Fatalf("total cats=%d want >=1", m.TotalCats)
	}
	// Heat on the catted memory must be positive; searches alone heat nothing.
	var heat float64
	if err := database.QueryRow(`SELECT heat FROM recall_stats WHERE uri = 'rmb://entities/heat-target'`).Scan(&heat); err != nil {
		t.Fatal(err)
	}
	if heat < 0.99 {
		t.Fatalf("heat=%v want >= 1 after cat", heat)
	}
}
