package debug

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/pipeline"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

// StuckSession is a pipeline row that appears wedged.
type StuckSession struct {
	SessionKey string `json:"session_key"`
	SessionURI string `json:"session_uri"`
	Stage      string `json:"stage"`
	Status     string `json:"status"`
	Since      string `json:"since,omitempty"`
	AgeSec     int64  `json:"age_sec"`
	AtomCount  int64  `json:"atom_count"`
	LastError  string `json:"last_error,omitempty"`
	LastStep   string `json:"last_step,omitempty"`
}

// StuckResponse is returned by GET /api/v1/debug/pipeline/stuck.
type StuckResponse struct {
	GeneratedAt string         `json:"generated_at"`
	OlderThan   string         `json:"older_than"`
	Items       []StuckSession `json:"items"`
}

// ListStuck returns sessions stuck in running or long-pending states.
func ListStuck(ctx context.Context, db *sql.DB, cfg config.PipelineConfig, olderThan time.Duration) (StuckResponse, error) {
	now := time.Now().UTC()
	cutoff := now.Add(-olderThan)
	out := StuckResponse{
		GeneratedAt: now.Format(time.RFC3339),
		OlderThan:   olderThan.String(),
		Items:       []StuckSession{},
	}

	running, err := listRunningStuck(ctx, db, cutoff)
	if err != nil {
		return StuckResponse{}, err
	}
	out.Items = append(out.Items, running...)

	pending, err := listPendingStuck(ctx, db, cfg, cutoff)
	if err != nil {
		return StuckResponse{}, err
	}
	out.Items = append(out.Items, pending...)

	return out, nil
}

func listRunningStuck(ctx context.Context, db *sql.DB, cutoff time.Time) ([]StuckSession, error) {
	cutoffMS := cutoff.UnixMilli()
	rows, err := db.QueryContext(ctx, `
		SELECT s.session_key, ps.l1_status, ps.l2_status, ps.l3_status,
			ps.l1_started_at, ps.l2_started_at, ps.l3_started_at,
			ps.l1_last_error, ps.l2_last_error, ps.l3_last_error,
			(SELECT COUNT(*) FROM atoms a WHERE a.session_id = s.id) AS atom_count
		FROM pipeline_state ps
		JOIN sessions s ON s.id = ps.session_id
		WHERE (ps.l1_status = 'running' AND ps.l1_started_at IS NOT NULL AND ps.l1_started_at < ?)
		   OR (ps.l2_status = 'running' AND ps.l2_started_at IS NOT NULL AND ps.l2_started_at < ?)
		   OR (ps.l3_status = 'running' AND ps.l3_started_at IS NOT NULL AND ps.l3_started_at < ?)`,
		cutoffMS, cutoffMS, cutoffMS,
	)
	if err != nil {
		return nil, fmt.Errorf("query running stuck: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	var items []StuckSession
	for rows.Next() {
		var sessionKey, l1, l2, l3 string
		var l1Start, l2Start, l3Start sql.NullInt64
		var l1Err, l2Err, l3Err sql.NullString
		var atomCount int64
		if err := rows.Scan(
			&sessionKey, &l1, &l2, &l3,
			&l1Start, &l2Start, &l3Start,
			&l1Err, &l2Err, &l3Err,
			&atomCount,
		); err != nil {
			return nil, err
		}
		for _, row := range []struct {
			stage, status string
			start         sql.NullInt64
			errText       sql.NullString
		}{
			{"t1", l1, l1Start, l1Err},
			{"t2", l2, l2Start, l2Err},
			{"t3", l3, l3Start, l3Err},
		} {
			if row.status != "running" || !row.start.Valid {
				continue
			}
			since := time.UnixMilli(row.start.Int64).UTC()
			items = append(items, StuckSession{
				SessionKey: sessionKey,
				SessionURI: uri.BuildSession(sessionKey),
				Stage:      row.stage,
				Status:     "running",
				Since:      since.Format(time.RFC3339),
				AgeSec:     int64(now.Sub(since).Seconds()),
				AtomCount:  atomCount,
				LastError:  nullString(row.errText),
				LastStep:   row.stage + ".running",
			})
		}
	}
	return items, rows.Err()
}

func listPendingStuck(ctx context.Context, db *sql.DB, cfg config.PipelineConfig, cutoff time.Time) ([]StuckSession, error) {
	cutoffMS := cutoff.UnixMilli()
	delayMS := int64(cfg.L2DelayAfterL1 / time.Millisecond)
	rows, err := db.QueryContext(ctx, `
		SELECT s.session_key, ps.l2_status, ps.l2_last_error, ps.l1_advanced_at, ps.updated_at,
			(SELECT COUNT(*) FROM atoms a WHERE a.session_id = s.id) AS atom_count
		FROM pipeline_state ps
		JOIN sessions s ON s.id = ps.session_id
		WHERE ps.l2_status = 'pending'
		  AND ps.l1_status != 'running'
		  AND ps.l1_advanced_at IS NOT NULL
		  AND ps.updated_at < ?
		  AND (? = 0 OR ps.l1_advanced_at + ? < ?)`,
		cutoffMS, delayMS, delayMS, time.Now().UTC().UnixMilli(),
	)
	if err != nil {
		return nil, fmt.Errorf("query pending stuck: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	var items []StuckSession
	for rows.Next() {
		var sessionKey, l2Status string
		var l2Err sql.NullString
		var l1Adv, updatedMS int64
		var atomCount int64
		if err := rows.Scan(&sessionKey, &l2Status, &l2Err, &l1Adv, &updatedMS, &atomCount); err != nil {
			return nil, err
		}
		since := time.UnixMilli(updatedMS).UTC()
		items = append(items, StuckSession{
			SessionKey: sessionKey,
			SessionURI: uri.BuildSession(sessionKey),
			Stage:      "t2",
			Status:     "pending",
			Since:      since.Format(time.RFC3339),
			AgeSec:     int64(now.Sub(since).Seconds()),
			AtomCount:  atomCount,
			LastError:  nullString(l2Err),
			LastStep:   "t2.waiting",
		})
	}
	return items, rows.Err()
}

// UnstickRequest is the body for POST /api/v1/debug/pipeline/unstick.
type UnstickRequest struct {
	ResetRunning bool   `json:"reset_running"`
	OlderThan    string `json:"older_than"`
	Stage        string `json:"stage"`
}

// UnstickResponse reports how many rows were reset.
type UnstickResponse struct {
	Reset map[string]int64 `json:"reset"`
}

// Unstick resets zombie running pipeline rows.
func Unstick(ctx context.Context, db *sql.DB, req UnstickRequest) (UnstickResponse, error) {
	out := UnstickResponse{Reset: map[string]int64{}}
	if !req.ResetRunning {
		return out, nil
	}

	olderThan, err := time.ParseDuration(strings.TrimSpace(req.OlderThan))
	if err != nil || olderThan <= 0 {
		olderThan = 5 * time.Minute
	}
	cutoffMS := time.Now().UTC().Add(-olderThan).UnixMilli()

	stages := []pipeline.Stage{pipeline.StageL1, pipeline.StageL2, pipeline.StageL3}
	if s := strings.TrimSpace(req.Stage); s != "" {
		stages = []pipeline.Stage{pipeline.Stage(strings.ToLower(s))}
	}

	for _, stage := range stages {
		n, err := pipeline.ResetRunningOlderThan(ctx, db, stage, cutoffMS)
		if err != nil {
			return UnstickResponse{}, err
		}
		out.Reset[string(stage)] = n
	}
	return out, nil
}

// RequeueRequest is the body for POST /api/v1/debug/pipeline/requeue.
type RequeueRequest struct {
	SessionKey string `json:"session_key"`
	Stage      string `json:"stage"`
}

// Requeue forces a session stage back to pending.
func Requeue(ctx context.Context, db *sql.DB, req RequeueRequest) error {
	sessionKey := strings.TrimSpace(req.SessionKey)
	if sessionKey == "" {
		return fmt.Errorf("session_key is required")
	}
	stage := pipeline.Stage(strings.ToLower(strings.TrimSpace(req.Stage)))
	switch stage {
	case pipeline.StageL1, pipeline.StageL2, pipeline.StageL3:
	default:
		return fmt.Errorf("stage must be l1, l2, or l3")
	}

	var sessionID string
	err := db.QueryRowContext(ctx, `SELECT id FROM sessions WHERE session_key = ?`, sessionKey).Scan(&sessionID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("session not found")
	}
	if err != nil {
		return err
	}
	return pipeline.Requeue(ctx, db, sessionID, stage)
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// ResolveSessionID looks up internal session id by session_key.
func ResolveSessionID(ctx context.Context, db *sql.DB, sessionKey string) (string, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return "", fmt.Errorf("session_key is required")
	}
	var sessionID string
	err := db.QueryRowContext(ctx, `SELECT id FROM sessions WHERE session_key = ?`, sessionKey).Scan(&sessionID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("session not found")
	}
	return sessionID, err
}
