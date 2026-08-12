package httpserver

import (
	"net/http"
	"strconv"

	"github.com/colinleefish/rmb-desktop/internal/browse"
	"github.com/colinleefish/rmb-desktop/internal/config"
)

const (
	defaultPageLimit = 25
	maxPageLimit     = 200
)

func parseListParams(r *http.Request) browse.ListParams {
	limit := defaultPageLimit
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	offset := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}
	return browse.ListParams{
		Limit:    limit,
		Offset:   offset,
		Query:    r.URL.Query().Get("q"),
		Category: r.URL.Query().Get("category"),
		Sort:     r.URL.Query().Get("sort"),
		Order:    r.URL.Query().Get("order"),
	}
}

func (s *Server) handleBrowseOverview(w http.ResponseWriter, r *http.Request) {
	out, err := s.browse.Overview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleBrowseSessions(w http.ResponseWriter, r *http.Request) {
	page, err := s.browse.ListSessions(r.Context(), parseListParams(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleBrowseSession(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("session_key")
	detail, err := s.browse.GetSession(r.Context(), key)
	if err != nil {
		if browse.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleBrowseAtoms(w http.ResponseWriter, r *http.Request) {
	page, err := s.browse.ListAtoms(r.Context(), parseListParams(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleBrowseScenes(w http.ResponseWriter, r *http.Request) {
	page, err := s.browse.ListScenes(r.Context(), parseListParams(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleBrowseMemories(w http.ResponseWriter, r *http.Request) {
	page, err := s.browse.ListMemories(r.Context(), parseListParams(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleBrowsePipelineHealth(w http.ResponseWriter, r *http.Request) {
	distillationEnabled := false
	if cfg, err := config.Load(s.configPath); err == nil {
		distillationEnabled = cfg.DistillationEnabled()
	}
	health, err := s.browse.PipelineHealth(r.Context(), distillationEnabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) handleBrowseTasks(w http.ResponseWriter, r *http.Request) {
	page, err := s.browse.ListTasks(r.Context(), parseListParams(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}
