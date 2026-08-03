package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/setup"
)

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	status, err := setup.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSetupPreview(w http.ResponseWriter, r *http.Request) {
	agent := strings.TrimSpace(r.PathValue("agent"))
	if agent == "" {
		writeError(w, http.StatusBadRequest, "agent required")
		return
	}
	state, err := setup.PreviewByName(agent)
	if err != nil {
		if strings.Contains(err.Error(), "unknown agent") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, setup.PreviewResponse{Agent: state})
}

func (s *Server) handleSetupApply(w http.ResponseWriter, r *http.Request) {
	agent := strings.TrimSpace(r.PathValue("agent"))
	if agent == "" {
		writeError(w, http.StatusBadRequest, "agent required")
		return
	}
	var req setup.ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Artifacts) == 0 {
		writeError(w, http.StatusBadRequest, "artifacts required")
		return
	}
	res, err := setup.Apply(agent, req.Artifacts)
	if err != nil {
		if strings.Contains(err.Error(), "unknown agent") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if strings.Contains(err.Error(), "copy-only") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
