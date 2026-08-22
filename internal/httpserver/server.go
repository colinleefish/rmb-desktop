package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/archive"
	"github.com/colinleefish/rmb-desktop/internal/browse"
	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/correction"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/http/static"
	"github.com/colinleefish/rmb-desktop/internal/inspect"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/recall"
	"github.com/colinleefish/rmb-desktop/internal/recallstats"
	"github.com/colinleefish/rmb-desktop/internal/session"
)

// Server is the rmbd HTTP API.
type Server struct {
	db          *sql.DB
	log         *slog.Logger
	reg         *debug.Registry
	logBuf      *debug.LogBuffer
	upload      *session.Service
	recall      *recall.Service
	recallStats *recallstats.Service
	inspect     *inspect.Service
	browse      *browse.Service
	corrections *correction.Service
	archive     *archive.Service
	embed       *llm.EmbeddingClient
	configPath  string
	mux         *http.ServeMux
}

func New(database *sql.DB, cfg config.Config, configPath string, log *slog.Logger, reg *debug.Registry, logBuf *debug.LogBuffer) *Server {
	if log == nil {
		log = slog.Default()
	}
	if reg == nil {
		reg = debug.NewRegistry()
	}
	if logBuf == nil {
		logBuf = debug.NewLogBuffer(0)
	}
	recallStatsSvc := recallstats.NewService(database)
	s := &Server{
		db:          database,
		log:         log,
		reg:         reg,
		logBuf:      logBuf,
		upload:      session.NewService(database),
		recall:      recall.NewService(database),
		recallStats: recallStatsSvc,
		inspect:     inspect.NewService(database),
		browse:      browse.NewService(database, recallStatsSvc),
		corrections: correction.NewService(database),
		archive:     archive.NewService(database),
		configPath:  configPath,
		mux:         http.NewServeMux(),
	}
	if cfg.Embed.HasKey() {
		if embedClient, err := llm.NewEmbeddingClient(cfg.Embed); err == nil {
			s.embed = embedClient
		} else {
			log.Warn("embed client disabled", "err", err)
		}
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /api/v1/version", s.handleVersion)
	s.mux.HandleFunc("POST /api/v1/sessions/{id}/upload", s.handleUpload)
	s.mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/v1/doctor/metrics", s.handleDoctorMetrics)
	s.mux.HandleFunc("GET /api/v1/doctor/archive", s.handleDoctorArchiveCandidates)
	s.mux.HandleFunc("POST /api/v1/doctor/archive", s.handleDoctorArchiveAction)
	s.mux.HandleFunc("POST /api/v1/maintenance/backfill-provenance", s.handleBackfillProvenance)
	s.mux.HandleFunc("GET /api/v1/inspect/cat", func(w http.ResponseWriter, r *http.Request) {
		s.handleInspect(w, r, "cat")
	})
	s.mux.HandleFunc("GET /api/v1/inspect/ls", func(w http.ResponseWriter, r *http.Request) {
		s.handleInspect(w, r, "ls")
	})
	s.mux.HandleFunc("GET /api/v1/inspect/meta", func(w http.ResponseWriter, r *http.Request) {
		s.handleInspect(w, r, "meta")
	})

	ui, err := static.UIHandler()
	if err != nil {
		s.log.Warn("ui static disabled", "err", err)
	} else {
		s.mux.Handle("GET /ui/{path...}", ui)
		s.mux.Handle("HEAD /ui/{path...}", ui)
		s.mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusFound)
		})
		s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/ui/", http.StatusFound)
		})
	}

	s.mux.HandleFunc("GET /api/v1/browse/overview", s.handleBrowseOverview)
	s.mux.HandleFunc("GET /api/v1/browse/sessions", s.handleBrowseSessions)
	s.mux.HandleFunc("GET /api/v1/browse/sessions/{session_key}", s.handleBrowseSession)
	s.mux.HandleFunc("GET /api/v1/browse/atoms", s.handleBrowseAtoms)
	s.mux.HandleFunc("GET /api/v1/browse/scenes", s.handleBrowseScenes)
	s.mux.HandleFunc("GET /api/v1/browse/memories", s.handleBrowseMemories)
	s.mux.HandleFunc("GET /api/v1/browse/skills", s.handleBrowseSkills)
	s.mux.HandleFunc("GET /api/v1/browse/skills/{slug}", s.handleBrowseSkill)
	s.mux.HandleFunc("PUT /api/v1/skills/{slug}", s.handlePutSkill)
	s.mux.HandleFunc("GET /api/v1/browse/pipeline-health", s.handleBrowsePipelineHealth)
	s.mux.HandleFunc("GET /api/v1/browse/tasks", s.handleBrowseTasks)
	s.mux.HandleFunc("GET /api/v1/corrections", s.handleListCorrections)
	s.mux.HandleFunc("POST /api/v1/corrections", s.handleCreateCorrection)
	s.mux.HandleFunc("DELETE /api/v1/corrections", s.handleRetractCorrection)
	s.mux.HandleFunc("GET /api/v1/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/v1/config", s.handlePutConfig)
	s.mux.HandleFunc("POST /api/v1/system/restart", s.handleRestart)
	s.mux.HandleFunc("POST /api/v1/config/test", s.handlePostConfigTest)
	s.mux.HandleFunc("POST /api/v1/config/test/llm", s.handlePostConfigTestLLM)
	s.mux.HandleFunc("POST /api/v1/config/test/embed", s.handlePostConfigTestEmbed)
	s.mux.HandleFunc("GET /api/v1/onboarding/status", s.handleOnboardingStatus)
	s.mux.HandleFunc("POST /api/v1/onboarding/complete", s.handleOnboardingComplete)
	s.mux.HandleFunc("POST /api/v1/onboarding/reset", s.handleOnboardingReset)
	s.mux.HandleFunc("GET /api/v1/setup/status", s.handleSetupStatus)
	s.mux.HandleFunc("GET /api/v1/setup/{agent}/preview", s.handleSetupPreview)
	s.mux.HandleFunc("POST /api/v1/setup/{agent}/apply", s.handleSetupApply)
	s.registerDebugRoutes()
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	_ = r.Context()
	sqliteVer, _ := db.SQLiteVersion(s.db)
	vecVer, vecErr := db.VecVersion(s.db)

	status := "ok"
	if vecErr != nil {
		status = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  status,
		"sqlite":  sqliteVer,
		"vec":     vecVer,
		"vec_ok":  vecErr == nil,
		"driver":  "mattn/go-sqlite3+cgo-vec",
		"checked": time.Now().UTC().Format(time.RFC3339),
	})
}

type uploadRequest struct {
	Messages  []session.Message `json:"messages"`
	Source    string            `json:"source"`
	StartedAt string            `json:"started_at"`
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	sessionKey := strings.TrimSpace(r.PathValue("id"))
	if sessionKey == "" {
		writeError(w, http.StatusBadRequest, "session id required")
		return
	}

	var req uploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	var startedAt *time.Time
	if s := strings.TrimSpace(req.StartedAt); s != "" {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "started_at must be RFC3339, e.g. 2026-05-09T17:00:00Z")
			return
		}
		startedAt = &ts
	}

	res, err := s.upload.Upload(r.Context(), session.UploadInput{
		SessionKey: sessionKey,
		Source:     req.Source,
		Messages:   req.Messages,
		StartedAt:  startedAt,
	})
	if err != nil {
		s.log.Error("upload failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"session_id": res.SessionID,
		"turn_id":    res.TurnID,
		"turn_uri":   res.TurnURI,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// ListenAndServe starts the HTTP server on addr.
func ListenAndServe(ctx context.Context, addr string, database *sql.DB, cfg config.Config, configPath string, log *slog.Logger, reg *debug.Registry, logBuf *debug.LogBuffer) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: New(database, cfg, configPath, log, reg, logBuf).Handler(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("rmbd listening", "addr", "http://"+addr)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
