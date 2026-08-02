package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/colinleefish/rmb-desktop/internal/config"
)

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, config.ToView(cfg, s.configPath))
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var req config.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	cfg, err := config.Load(s.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	updated, err := config.ApplyUpdate(cfg, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.Save(s.configPath, updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("config saved", "path", s.configPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Config saved. Restart rmbd to apply LLM/embed worker changes.",
		"config":  config.ToView(updated, s.configPath),
	})
}
