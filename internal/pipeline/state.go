package pipeline

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	rdb "github.com/colinleefish/rmb-desktop/internal/db"
)

// Stage is a distillation tier column prefix in pipeline_state.
type Stage string

const (
	StageL1 Stage = "l1"
	StageL2 Stage = "l2"
	StageL3 Stage = "l3"
)

func (s Stage) statusCol() string  { return string(s) + "_status" }
func (s Stage) startedCol() string { return string(s) + "_started_at" }
func (s Stage) errorCol() string   { return string(s) + "_last_error" }

// MarkRunning sets stage status to running and records started_at.
func MarkRunning(ctx context.Context, db *sql.DB, sessionID string, stage Stage, now time.Time) error {
	nowMS := now.UTC().UnixMilli()
	query := fmt.Sprintf(`
		UPDATE pipeline_state SET
			%s = 'running',
			%s = ?,
			%s = NULL,
			updated_at = ?
		WHERE session_id = ?`,
		stage.statusCol(), stage.startedCol(), stage.errorCol(),
	)
	_, err := db.ExecContext(ctx, query, nowMS, nowMS, sessionID)
	return err
}

// MarkPending resets stage to pending (transient failure).
func MarkPending(ctx context.Context, db *sql.DB, sessionID string, stage Stage, errMsg string, now time.Time) error {
	nowMS := now.UTC().UnixMilli()
	query := fmt.Sprintf(`
		UPDATE pipeline_state SET
			%s = 'pending',
			%s = NULL,
			%s = ?,
			updated_at = ?
		WHERE session_id = ?`,
		stage.statusCol(), stage.startedCol(), stage.errorCol(),
	)
	_, err := db.ExecContext(ctx, query, rdb.NullIfEmpty(errMsg), nowMS, sessionID)
	return err
}

// MarkFailed marks a permanent stage failure.
func MarkFailed(ctx context.Context, db *sql.DB, sessionID string, stage Stage, errMsg string, now time.Time) error {
	nowMS := now.UTC().UnixMilli()
	query := fmt.Sprintf(`
		UPDATE pipeline_state SET
			%s = 'failed',
			%s = ?,
			updated_at = ?
		WHERE session_id = ?`,
		stage.statusCol(), stage.errorCol(),
	)
	_, err := db.ExecContext(ctx, query, rdb.NullIfEmpty(errMsg), nowMS, sessionID)
	return err
}

// ClearRunning clears started_at after successful completion (status already updated).
func ClearRunning(ctx context.Context, db *sql.DB, sessionID string, stage Stage) error {
	query := fmt.Sprintf(`UPDATE pipeline_state SET %s = NULL, %s = NULL WHERE session_id = ?`,
		stage.startedCol(), stage.errorCol(),
	)
	_, err := db.ExecContext(ctx, query, sessionID)
	return err
}

// ResetRunningToPending clears zombie running rows (startup recovery / unstick).
func ResetRunningToPending(ctx context.Context, db *sql.DB, stage Stage) (int64, error) {
	query := fmt.Sprintf(`
		UPDATE pipeline_state SET
			%s = 'pending',
			%s = NULL,
			updated_at = ?
		WHERE %s = 'running'`,
		stage.statusCol(), stage.startedCol(), stage.statusCol(),
	)
	res, err := db.ExecContext(ctx, query, time.Now().UTC().UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ResetRunningOlderThan resets running rows older than cutoffMS back to pending.
func ResetRunningOlderThan(ctx context.Context, db *sql.DB, stage Stage, cutoffMS int64) (int64, error) {
	query := fmt.Sprintf(`
		UPDATE pipeline_state SET
			%s = 'pending',
			%s = NULL,
			updated_at = ?
		WHERE %s = 'running' AND %s IS NOT NULL AND %s < ?`,
		stage.statusCol(), stage.startedCol(),
		stage.statusCol(), stage.startedCol(), stage.startedCol(),
	)
	nowMS := time.Now().UTC().UnixMilli()
	res, err := db.ExecContext(ctx, query, nowMS, cutoffMS)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Requeue sets a stage back to pending for one session.
func Requeue(ctx context.Context, db *sql.DB, sessionID string, stage Stage) error {
	nowMS := time.Now().UTC().UnixMilli()
	query := fmt.Sprintf(`
		UPDATE pipeline_state SET
			%s = 'pending',
			%s = NULL,
			%s = NULL,
			updated_at = ?
		WHERE session_id = ?`,
		stage.statusCol(), stage.startedCol(), stage.errorCol(),
	)
	_, err := db.ExecContext(ctx, query, nowMS, sessionID)
	return err
}
