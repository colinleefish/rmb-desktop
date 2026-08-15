// Package shared holds plumbing common to the stage workers (L1/L2/L3/embed):
// stage-error marking and the poll loop that drives each worker's cycle. It
// lives beside the workers rather than inside them to avoid import cycles
// (internal/worker imports each worker package).
package shared

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/pipeline"
)

// MarkProcessError records a failed stage attempt: transient errors (per
// llm.IsTransientError) log a warning and reset the stage to pending so it is
// retried on a later cycle; permanent errors mark the stage failed. The
// original error is returned unchanged for upstream handling.
func MarkProcessError(ctx context.Context, db *sql.DB, log *slog.Logger, stage pipeline.Stage, sessionID string, cause error, now time.Time) error {
	if llm.IsTransientError(cause) {
		log.Warn(string(stage)+" transient error", "session_id", sessionID, "err", cause)
		_ = pipeline.MarkPending(ctx, db, sessionID, stage, cause.Error(), now)
		return cause
	}
	_ = pipeline.MarkFailed(ctx, db, sessionID, stage, cause.Error(), now)
	return cause
}
