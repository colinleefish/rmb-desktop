package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/colinleefish/rmb-desktop/internal/onboarding"
)

type onboardingStatusResponse struct {
	Completed     bool   `json:"completed"`
	MarkerPath    string `json:"marker_path"`
	CompletedAt   string `json:"completed_at,omitempty"`
	SkippedAgents bool   `json:"skipped_agents"`
}

type onboardingCompleteRequest struct {
	SkippedAgents bool `json:"skipped_agents"`
}

func (s *Server) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	completed, marker, path, err := onboarding.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, onboardingStatusResponse{
		Completed:     completed,
		MarkerPath:    path,
		CompletedAt:   marker.CompletedAt,
		SkippedAgents: marker.SkippedAgents,
	})
}

func (s *Server) handleOnboardingComplete(w http.ResponseWriter, r *http.Request) {
	var req onboardingCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	path, err := onboarding.MarkComplete(req.SkippedAgents)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("onboarding complete", "marker", path, "skipped_agents", req.SkippedAgents)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"marker_path": path,
	})
}

func (s *Server) handleOnboardingReset(w http.ResponseWriter, r *http.Request) {
	if err := onboarding.Reset(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("onboarding reset")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
