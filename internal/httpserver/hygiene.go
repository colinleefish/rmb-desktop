package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/hygiene"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/worker/memory"
)

// handleDoctorReport composes the full health snapshot (issue #33): the
// deterministic hygiene scans, the archival proposal count (#32), and the
// retrieval-health metrics (#24) in one response. Purely read-only.
func (s *Server) handleDoctorReport(w http.ResponseWriter, r *http.Request) {
	type reportResponse struct {
		Hygiene           hygiene.HealthReport `json:"hygiene"`
		ArchiveCandidates int                  `json:"archive_candidates"`
		ArchiveEnabled    bool                 `json:"archive_enabled"`
		ZeroCatSearchRate float64              `json:"zero_cat_search_rate"`
		HeatConcentration float64              `json:"heat_concentration"`
		HeatAlarm         bool                 `json:"heat_alarm"`
	}
	out := reportResponse{}

	rep, err := hygiene.Report(r.Context(), s.db, 50)
	if err != nil {
		s.log.Error("doctor report failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Hygiene = rep

	if s.archive != nil {
		out.ArchiveEnabled = true
		if cands, err := s.archive.Candidates(r.Context(), 0); err == nil {
			out.ArchiveCandidates = len(cands)
		}
	}
	if s.recallStats != nil {
		if m, err := s.recallStats.DoctorMetrics(r.Context()); err == nil {
			out.ZeroCatSearchRate = m.ZeroCatRate
			out.HeatConcentration = m.HeatConcentration
			out.HeatAlarm = m.HeatAlarm
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleDoctorGC garbage-collects superseded rows (issue #33): per URI keep
// the newer of (keep_versions newest, keep_days freshest). POST with
// dry_run=true (default) reports; dry_run=false deletes SUPERSEDED rows
// only — active memories are never touched.
func (s *Server) handleDoctorGC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DryRun       *bool `json:"dry_run"`
		KeepVersions int   `json:"keep_versions"`
		KeepDays     int   `json:"keep_days"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
	}
	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	if strings.EqualFold(r.URL.Query().Get("dry_run"), "false") {
		dryRun = false
	}
	stats, err := hygiene.SupersededGC(r.Context(), s.db, req.KeepVersions, req.KeepDays, dryRun)
	if err != nil {
		s.log.Error("doctor gc failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// handleDebugReconsolidate rebuilds a memory from its atom evidence
// (issue #33 / plan §9.3e): POST /api/v1/debug/pipeline/reconsolidate
// {"uri": "rmb://entities/aliyun"}. Heavy (LLM distill); debug-gated like
// the dry-run endpoints.
func (s *Server) handleDebugReconsolidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URI           string `json:"uri"`
		PromptVersion int    `json:"prompt_version"`
	}
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
	chat, err := llm.NewOpenAICompatibleClient(cfg.LLM, s.log)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.PromptVersion != 0 {
		if err := chat.SetDistillPromptVersion(req.PromptVersion); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), llm.DebugRequestBudget(cfg.LLM, 8))
	defer cancel()
	result, err := memory.Reconsolidate(ctx, s.db, chat, cfg.Pipeline, s.log, req.URI)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
