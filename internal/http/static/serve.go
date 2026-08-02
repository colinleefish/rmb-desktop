package static

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// UIHandler serves the embedded SPA under /ui/.
func UIHandler() (http.Handler, error) {
	webFS, err := fs.Sub(Web, "web")
	if err != nil {
		return nil, err
	}
	fileServer := http.StripPrefix("/ui", http.FileServer(http.FS(webFS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/ui")
		if rel == "" {
			rel = "/"
		}
		if rel == "/" || rel == "/index.html" || strings.HasPrefix(rel, "/assets/") || strings.Contains(path.Base(rel), ".") {
			if rel == "/index.html" {
				serveAtUIPath(w, r, fileServer, "/ui/")
				return
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := webFS.Open(strings.TrimPrefix(rel, "/")); err != nil {
			serveAtUIPath(w, r, fileServer, "/ui/")
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func serveAtUIPath(w http.ResponseWriter, r *http.Request, h http.Handler, uiPath string) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = uiPath
	h.ServeHTTP(w, r2)
}
