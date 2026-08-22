package httpserver

import (
	"net/http"
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
