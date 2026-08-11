package httpserver

import (
	"net/http"

	"github.com/colinleefish/rmb-desktop/internal/version"
)

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
	})
}
