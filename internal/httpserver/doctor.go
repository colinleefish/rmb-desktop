package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// handleDoctorMetrics reports retrieval-health signals (issue #24):
// zero-cat search rate and heat concentration (feedback-loop alarm).
// Read-only; derived from local telemetry only.
func (s *Server) handleDoctorMetrics(w http.ResponseWriter, r *http.Request) {
	if s.recallStats == nil {
		writeError(w, http.StatusServiceUnavailable, "recall stats unavailable")
		return
	}
	m, err := s.recallStats.DoctorMetrics(r.Context())
	if err != nil {
		s.log.Error("doctor metrics failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// handleDoctorArchiveCandidates is the doctor's reviewable proposal (issue
// #32): the cold memories that meet the archival policy. Purely read-only —
// use POST with action=archive to apply. ?days= overrides the window.
func (s *Server) handleDoctorArchiveCandidates(w http.ResponseWriter, r *http.Request) {
	days := 0
	if v := strings.TrimSpace(r.URL.Query().Get("days")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "days must be an integer")
			return
		}
		days = n
	}
	cands, err := s.archive.Candidates(r.Context(), days)
	if err != nil {
		s.log.Error("doctor archive candidates failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"candidates": cands,
		"count":      len(cands),
	})
}

// handleDoctorArchiveAction applies (or restores) archival per the user's
// explicit request. Body: {"action":"archive"|"restore", "uris":[...]|null,
// "all":bool}. An empty uris list on archive bulk-archives the proposed set;
// restore requires uris or all=true. Nothing is ever deleted.
func (s *Server) handleDoctorArchiveAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string   `json:"action"`
		URIs   []string `json:"uris"`
		All    bool     `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	nowMS := time.Now().UTC().UnixMilli()
	switch req.Action {
	case "archive":
		n, err := s.archive.Apply(r.Context(), req.URIs, nowMS)
		if err != nil {
			s.log.Error("archive apply failed", "err", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"archived": n})
	case "restore":
		n, err := s.archive.Restore(r.Context(), req.URIs, req.All)
		if err != nil {
			s.log.Error("archive restore failed", "err", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"restored": n})
	default:
		writeError(w, http.StatusBadRequest, "action must be archive or restore")
	}
}
