package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/skill"
)

const maxSkillUploadBytes = 32 << 20 // 32 MiB

func (s *Server) handleBrowseSkills(w http.ResponseWriter, r *http.Request) {
	page, err := s.browse.ListSkills(r.Context(), parseListParams(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleBrowseSkill(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug required")
		return
	}
	detail, err := s.browse.GetSkill(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handlePutSkill(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSkillUploadBytes)
	var req struct {
		Files []skill.FileInput `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "files array is required")
		return
	}
	if len(req.Files) == 0 {
		writeError(w, http.StatusBadRequest, "files array is required")
		return
	}

	result, err := skill.ReplaceBundle(r.Context(), s.db, slug, skill.BundleInput{Files: req.Files})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
