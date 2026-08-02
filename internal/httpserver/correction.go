package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/correction"
)

func (s *Server) handleCreateCorrection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURIs []string `json:"target_uris"`
		Statement  string   `json:"statement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	row, err := s.corrections.Create(r.Context(), correction.CreateInput{
		TargetURIs: req.TargetURIs,
		Statement:  req.Statement,
	})
	if err != nil {
		writeCorrectionError(w, err)
		return
	}

	if _, err := correction.EnqueueSessionsForMemoryTargets(r.Context(), s.db, req.TargetURIs); err != nil {
		s.log.Warn("correction wake-l3 failed (overlay still applies)", "err", err)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"uri":         row.URI,
		"statement":   row.Statement,
		"created_at":  row.CreatedAt,
		"target_uris": req.TargetURIs,
	})
}

func (s *Server) handleListCorrections(w http.ResponseWriter, r *http.Request) {
	p := parseListParams(r)
	items, total, err := s.corrections.List(r.Context(), r.URL.Query().Get("target"), p.Limit, p.Offset)
	if err != nil {
		writeCorrectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  p.Limit,
		"offset": p.Offset,
	})
}

func (s *Server) handleRetractCorrection(w http.ResponseWriter, r *http.Request) {
	uri := strings.TrimSpace(r.URL.Query().Get("uri"))
	if uri == "" {
		writeError(w, http.StatusBadRequest, "uri is required")
		return
	}

	targets, err := s.corrections.Retract(r.Context(), uri)
	if err != nil {
		writeCorrectionError(w, err)
		return
	}

	if _, err := correction.EnqueueSessionsForMemoryTargets(r.Context(), s.db, targets); err != nil {
		slog.Default().Warn("correction wake-l3 failed after retract", "err", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"uri":       uri,
		"retracted": true,
	})
}

func writeCorrectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, correction.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, correction.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
