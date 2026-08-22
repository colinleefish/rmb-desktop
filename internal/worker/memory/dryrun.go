package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
)

// L3DryRunBucket is one bucket in an L3 dry-run trace.
type L3DryRunBucket struct {
	URI              string `json:"uri"`
	Category         string `json:"category"`
	Slug             string `json:"slug,omitempty"`
	AtomCount        int    `json:"atom_count"`
	DistinctSessions int    `json:"distinct_sessions"`
	RelatedCount     int    `json:"related_count"`
	// RelatedURIs are the retrieve-then-link candidates injected for event
	// buckets (P2.2 / issue #28).
	RelatedURIs []string `json:"related_uris,omitempty"`
	// Decision is one of: "distilled", "graduation-deferred", "unchanged".
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	// Existing* hold the currently active memory (before), if any.
	ExistingAbstract string `json:"existing_abstract,omitempty"`
	ExistingBody     string `json:"existing_body,omitempty"`
	// Abstract/Body are what this prompt generation WOULD produce (after).
	Abstract string `json:"abstract,omitempty"`
	Body     string `json:"body,omitempty"`
	MS       int64  `json:"ms"`
}

// L3DryRunResult is returned by the L3 dry-run endpoint.
type L3DryRunResult struct {
	SessionKey    string           `json:"session_key"`
	SessionID     string           `json:"session_id"`
	AtomCount     int              `json:"atom_count"`
	BucketCount   int              `json:"bucket_count"`
	PromptVersion int              `json:"prompt_version"`
	Buckets       []L3DryRunBucket `json:"buckets"`
	Error         string           `json:"error,omitempty"`
}

// DryRunL3 runs the L3 distill pipeline for one session's atoms WITHOUT
// persisting anything. Each bucket reports the graduation decision, the
// related-event link candidates, and the body the current prompt generation
// would produce next to the existing active memory — the primitive used to
// A/B replay real sessions (issue #28) before rolling a prompt out.
func DryRunL3(
	ctx context.Context,
	database *sql.DB,
	distiller MemoryDistiller,
	cfg config.PipelineConfig,
	log *slog.Logger,
	sessionID string,
) (*L3DryRunResult, error) {
	var sessionKey string
	err := database.QueryRowContext(ctx, `SELECT session_key FROM sessions WHERE id = ?`, sessionID).Scan(&sessionKey)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, err
	}

	if log == nil {
		log = slog.Default()
	}

	atoms, err := loadSessionAtoms(ctx, database, sessionID)
	if err != nil {
		return nil, err
	}
	buckets, skipped := groupAtomsIntoBuckets(atoms)
	if skipped > 0 {
		log.Warn("l3 dry-run skipped slug-less atoms", "count", skipped)
	}

	result := &L3DryRunResult{
		SessionKey:  sessionKey,
		SessionID:   sessionID,
		AtomCount:   len(atoms),
		BucketCount: len(buckets),
		Buckets:     make([]L3DryRunBucket, 0, len(buckets)),
	}

	w := NewWorker(database, distiller, nil, cfg, log, nil)

	for _, bucket := range buckets {
		out := L3DryRunBucket{
			URI:              bucket.URI,
			Category:         bucket.Category,
			Slug:             bucket.Slug,
			AtomCount:        len(bucket.Atoms),
			DistinctSessions: distinctSessionCount(bucket.Atoms),
		}
		start := time.Now()

		existingAbstract, existingBody := activeMemoryBody(ctx, database, bucket.URI)
		out.ExistingAbstract = existingAbstract
		out.ExistingBody = existingBody

		switch {
		case bucket.Category == model.AtomCategoryEvents:
			// Events are immutable append-only records: dry-run shows what a
			// fresh distill of the same atoms would produce, so the related
			// links and structured body are visible even when a memory
			// already exists.
			out.Decision = "distilled"
		default:
			deferred, err := w.graduationDeferred(ctx, bucket)
			if err != nil {
				out.Decision = "error"
				out.Reason = err.Error()
				out.MS = time.Since(start).Milliseconds()
				result.Buckets = append(result.Buckets, out)
				continue
			}
			if deferred {
				out.Decision = "graduation-deferred"
				out.Reason = fmt.Sprintf("below corroboration bar: %d distinct session(s) < %d; atoms kept append-only",
					out.DistinctSessions, graduationMinSessions)
				out.MS = time.Since(start).Milliseconds()
				result.Buckets = append(result.Buckets, out)
				continue
			}
			// Report the REAL rollup gate decision (P2.1 materiality: the
			// active row's atom fingerprint), not just row existence.
			unchanged, err := w.bucketUnchanged(ctx, bucket, nil, nil)
			if err != nil {
				out.Decision = "error"
				out.Reason = err.Error()
			} else if unchanged {
				out.Decision = "unchanged"
				out.Reason = "materiality gate: atom fingerprint unchanged since last distill"
			} else {
				out.Decision = "distilled"
			}
		}

		pm, related, err := w.distillBucketTrace(ctx, bucket)
		if err != nil {
			out.Decision = "error"
			out.Reason = err.Error()
			out.MS = time.Since(start).Milliseconds()
			result.Buckets = append(result.Buckets, out)
			continue
		}
		out.Abstract = pm.Abstract
		out.Body = pm.Body
		out.RelatedCount = len(related)
		out.RelatedURIs = make([]string, 0, len(related))
		for _, r := range related {
			out.RelatedURIs = append(out.RelatedURIs, r.URI)
		}
		out.MS = time.Since(start).Milliseconds()
		result.Buckets = append(result.Buckets, out)
	}
	return result, nil
}

// distillBucketTrace runs distillBucket and also surfaces the related-event
// candidates considered for event buckets (for dry-run reporting).
func (w *Worker) distillBucketTrace(ctx context.Context, bucket Bucket) (ParsedMemory, []llm.RelatedEvent, error) {
	pm, err := w.distillBucket(ctx, bucket, nil)
	if err != nil {
		return ParsedMemory{}, nil, err
	}
	if bucket.Category != model.AtomCategoryEvents {
		return pm, nil, nil
	}
	related, err := w.relatedEvents(ctx, bucket)
	if err != nil {
		return pm, nil, nil
	}
	return pm, related, nil
}

func activeMemoryBody(ctx context.Context, database *sql.DB, uri string) (abstract, body string) {
	var a, b sql.NullString
	err := database.QueryRowContext(ctx, `
		SELECT abstract, body FROM memories WHERE uri = ? AND superseded_at IS NULL`, uri,
	).Scan(&a, &b)
	if err != nil {
		return "", ""
	}
	if a.Valid {
		abstract = a.String
	}
	if b.Valid {
		body = b.String
	}
	return abstract, body
}

func loadSessionAtoms(ctx context.Context, database *sql.DB, sessionID string) ([]model.Atom, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at
		FROM atoms WHERE session_id = ? ORDER BY created_at ASC, id ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return db.ScanAtomRows(rows)
}
