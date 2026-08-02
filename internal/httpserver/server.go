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

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/session"
)

// Server is the rmbd HTTP API.
type Server struct {
	db     *sql.DB
	log    *slog.Logger
	upload *session.Service
	mux    *http.ServeMux
}

func New(database *sql.DB, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		db:     database,
		log:    log,
		upload: session.NewService(database),
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /api/v1/sessions/{id}/upload", s.handleUpload)
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
	Messages []session.Message `json:"messages"`
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

	res, err := s.upload.Upload(r.Context(), session.UploadInput{
		SessionKey: sessionKey,
		Messages:   req.Messages,
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
func ListenAndServe(ctx context.Context, addr string, database *sql.DB, log *slog.Logger) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: New(database, log).Handler(),
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
