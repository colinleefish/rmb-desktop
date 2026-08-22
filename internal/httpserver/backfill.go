package httpserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/backfill"
)

// handleBackfillProvenance runs the one-time, idempotent provenance backfill
// (issue #31, RC7) against the local database. Local maintenance only — it
// reads existing data and writes backfilled source_scene_uris where
// recoverable. Query params: threshold (default 0.9), max-scenes (default 5),
// dry-run (true to report only), categories (comma list, optional).
func (s *Server) handleBackfillProvenance(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	opts := backfill.Options{}
	if v, err := strconv.ParseFloat(q.Get("threshold"), 64); err == nil {
		opts.Threshold = v
	}
	if v, err := strconv.Atoi(q.Get("max-scenes")); err == nil {
		opts.MaxScenes = v
	}
	if v, err := strconv.ParseBool(q.Get("dry-run")); err == nil {
		opts.DryRun = v
	}
	if raw := q.Get("categories"); raw != "" {
		for _, c := range strings.Split(raw, ",") {
			if c = strings.TrimSpace(c); c != "" {
				opts.Categories = append(opts.Categories, c)
			}
		}
	}

	stats, err := backfill.BackfillProvenance(r.Context(), s.db, opts)
	if err != nil {
		s.log.Error("backfill provenance failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
