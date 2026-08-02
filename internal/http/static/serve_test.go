package static

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddedWebHasIndex(t *testing.T) {
	webFS, err := fs.Sub(Web, "web")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	data, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("index.html empty")
	}
}

func TestUIHandlerServesIndex(t *testing.T) {
	h, err := UIHandler()
	if err != nil {
		t.Fatalf("UIHandler: %v", err)
	}

	cases := []struct {
		path string
		want int
	}{
		{"/ui/", http.StatusOK},
		{"/ui/index.html", http.StatusOK},
		{"/ui/sessions", http.StatusOK},
		{"/ui/nonexistent/", http.StatusOK},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("GET %s = %d (location=%q), want %d", tc.path, w.Code, w.Header().Get("Location"), tc.want)
		}
	}
}
