package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/worker/scene"
)

func (s *Server) registerDebugRoutes() {
	s.mux.HandleFunc("GET /api/v1/debug/workers", s.handleDebugWorkers)
	s.mux.HandleFunc("GET /api/v1/debug/in-flight", s.handleDebugInFlight)
	s.mux.HandleFunc("GET /api/v1/debug/backpressure", s.handleDebugBackpressure)
	s.mux.HandleFunc("GET /api/v1/debug/logs", s.handleDebugLogs)
	s.mux.HandleFunc("GET /api/v1/debug/sqlite", s.handleDebugSQLite)
	s.mux.HandleFunc("GET /api/v1/debug/pipeline/stuck", s.handleDebugPipelineStuck)
	s.mux.HandleFunc("POST /api/v1/debug/pipeline/unstick", s.handleDebugPipelineUnstick)
	s.mux.HandleFunc("POST /api/v1/debug/pipeline/requeue", s.handleDebugPipelineRequeue)
	s.mux.HandleFunc("POST /api/v1/debug/pipeline/dry-run", s.handleDebugPipelineDryRun)
	s.mux.HandleFunc("POST /api/v1/debug/llm/build-scenes", s.handleDebugLLMBuildScenes)
}

func (s *Server) handleDebugWorkers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.reg.Snapshot())
}

func (s *Server) handleDebugInFlight(w http.ResponseWriter, r *http.Request) {
	snap := s.reg.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{"in_flight": snap.InFlight})
}

func (s *Server) handleDebugBackpressure(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.reg.BackpressureSnapshot())
}

func (s *Server) handleDebugLogs(w http.ResponseWriter, r *http.Request) {
	tail := 200
	if v := strings.TrimSpace(r.URL.Query().Get("tail")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			tail = n
		}
	}
	entries := s.logBuf.Tail(tail, r.URL.Query().Get("level"), r.URL.Query().Get("worker"))
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"count":   len(entries),
	})
}

func (s *Server) handleDebugSQLite(w http.ResponseWriter, r *http.Request) {
	stats, err := debug.CollectSQLiteStats(r.Context(), s.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleDebugPipelineStuck(w http.ResponseWriter, r *http.Request) {
	olderThan := 5 * time.Minute
	if v := strings.TrimSpace(r.URL.Query().Get("older_than")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			olderThan = d
		}
	}
	cfg, err := s.loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out, err := debug.ListStuck(r.Context(), s.db, cfg.Pipeline, olderThan)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDebugPipelineUnstick(w http.ResponseWriter, r *http.Request) {
	var req debug.UnstickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !req.ResetRunning {
		req.ResetRunning = true
	}
	out, err := debug.Unstick(r.Context(), s.db, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDebugPipelineRequeue(w http.ResponseWriter, r *http.Request) {
	var req debug.RequeueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := debug.Requeue(r.Context(), s.db, req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type dryRunRequest struct {
	SessionKey string `json:"session_key"`
	Stage      string `json:"stage"`
}

func (s *Server) handleDebugPipelineDryRun(w http.ResponseWriter, r *http.Request) {
	var req dryRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	stage := strings.ToLower(strings.TrimSpace(req.Stage))
	if stage == "" {
		stage = "t2"
	}
	if stage != "t2" {
		writeError(w, http.StatusBadRequest, "only stage t2 is supported")
		return
	}

	cfg, err := s.loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !cfg.DistillationEnabled() {
		writeError(w, http.StatusBadRequest, "distillation disabled (no llm api key)")
		return
	}

	sessionID, err := debug.ResolveSessionID(r.Context(), s.db, req.SessionKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	chat, err := llm.NewOpenAICompatibleClient(cfg.LLM)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	result, err := scene.DryRunT2(ctx, s.db, chat, cfg.Pipeline, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type buildScenesRequest struct {
	SessionKey string `json:"session_key"`
	AtomsJSON  string `json:"atoms_json"`
}

func (s *Server) handleDebugLLMBuildScenes(w http.ResponseWriter, r *http.Request) {
	var req buildScenesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	cfg, err := s.loadConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !cfg.DistillationEnabled() {
		writeError(w, http.StatusBadRequest, "distillation disabled (no llm api key)")
		return
	}

	atomsJSON := strings.TrimSpace(req.AtomsJSON)
	if atomsJSON == "" && strings.TrimSpace(req.SessionKey) != "" {
		sessionID, err := debug.ResolveSessionID(r.Context(), s.db, req.SessionKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		atomsJSON, err = debug.SerializeSessionAtoms(r.Context(), s.db, sessionID, cfg.Pipeline)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if atomsJSON == "" {
		writeError(w, http.StatusBadRequest, "session_key or atoms_json is required")
		return
	}

	chat, err := llm.NewOpenAICompatibleClient(cfg.LLM)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	raw, scenes, steps, err := scene.BuildScenesProbe(ctx, chat, atomsJSON)
	out := map[string]any{
		"steps":       steps,
		"scene_count": len(scenes),
		"raw_preview": previewText(raw, 2000),
	}
	if err != nil {
		out["error"] = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) loadConfig() (config.Config, error) {
	if s.configPath == "" {
		return config.Config{}, nil
	}
	return config.Load(s.configPath)
}

func previewText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
