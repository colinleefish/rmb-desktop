package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/httpserver"
)

func TestUpload_roundTrip(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	cfg, _ := config.Default()
	srv := httpserver.New(database, cfg, filepath.Join(t.TempDir(), "config.yaml"), nil)

	body, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": "hi there"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/upload", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var count int
	if err := database.QueryRow(`SELECT count(*) FROM session_turns`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("turn count=%d", count)
	}
}
