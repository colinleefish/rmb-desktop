// Package backfill provides one-time, idempotent data repairs for the memory
// pyramid. Issue #31 / plan §5 P3.2, RC7: ~37.5% of visible events carry empty
// source_scene_uris, so provenance cannot be walked from a memory down to its
// evidence scenes.
//
// Recovery principle: "link, don't guess; similarity only where no link
// exists, with calibrated thresholds" (plan). BackfillProvenance reads existing
// data only — it never calls the LLM and never touches exposed rows. For each
// memory with empty source_scene_uris it links the scenes whose *distilled
// bodies* (themselves the distillation of those scenes' atoms — i.e. the
// recovery is grounded in the atom evidence tier) are cosine-similar to the
// memory above a calibrated threshold.
package backfill

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
)

// ProveQueryEmbeddingThreshold: a scene qualifies as a memory's provenance when
// cosine similarity ≥ this value. Calibrated lenient (0.90) because we only
// backfill rows that are otherwise orphaned; strict watermarking (P3.1) is a
// follow-up for ambiguous cases.
const ProveQueryEmbeddingThreshold = 0.90

// DefaultMaxScenes caps how many provenance scenes a backfilled memory gets.
const DefaultMaxScenes = 5

// Stats reports the outcome of a backfill pass.
type Stats struct {
	MemoriesScanned int      `json:"memories_scanned"`
	MemoriesLinked  int      `json:"memories_linked"`
	ScenesLinked    int      `json:"scenes_linked"`
	DryRun          bool     `json:"dry_run"`
	Massed          []string `json:"unresolved_uris,omitempty"` // debug aid
}

// Options controls a backfill pass.
type Options struct {
	Threshold float64 // min cosine similarity to link a scene (default 0.90)
	MaxScenes int     // max scenes per memory (default 5)
	DryRun    bool    // if true, only report; do not write
	// Categories limits which memory categories are backfilled (empty = all
	// with empty source_scene_uris).
	Categories []string
}

// BackfillProvenance walks every active memory with empty source_scene_uris
// and, where recoverable, sets its provenance to the up-to-MaxScenes scenes
// most similar to it (cosine ≥ Threshold). It is idempotent: rows already
// having provenance are skipped, and deterministic inputs yield the same
// result on re-run.
func BackfillProvenance(ctx context.Context, database *sql.DB, opts Options) (*Stats, error) {
	if opts.Threshold <= 0 {
		opts.Threshold = ProveQueryEmbeddingThreshold
	}
	if opts.MaxScenes <= 0 {
		opts.MaxScenes = DefaultMaxScenes
	}

	stats := &Stats{DryRun: opts.DryRun}

	categorySQL := ""
	var catArgs []any
	if len(opts.Categories) > 0 {
		categorySQL = ` AND category IN (`
		for i, c := range opts.Categories {
			if i > 0 {
				categorySQL += ","
			}
			categorySQL += "?"
			catArgs = append(catArgs, c)
		}
		categorySQL += ")"
	}

	memRows, err := database.QueryContext(ctx, `
		SELECT uri, embedding FROM memories
		WHERE superseded_at IS NULL
		  AND (source_scene_uris = '[]' OR source_scene_uris IS NULL OR TRIM(source_scene_uris) = '')
		  AND embedding IS NOT NULL`+categorySQL, catArgs...)
	if err != nil {
		return nil, fmt.Errorf("scan memories: %w", err)
	}

	// Materialize candidates first so no read cursor is held open while we
	// write (single-writer WAL; an open cursor on the same pool connection
	// would self-deadlock the UPDATE below).
	type cand struct {
		uri  string
		blob []byte
	}
	var candidates []cand
	for memRows.Next() {
		var c cand
		if err := memRows.Scan(&c.uri, &c.blob); err != nil {
			memRows.Close()
			return nil, err
		}
		candidates = append(candidates, c)
	}
	if err := memRows.Close(); err != nil {
		return nil, err
	}

	for _, c := range candidates {
		stats.MemoriesScanned++

		scenes, err := bestScenes(ctx, database, c.blob, opts.Threshold, opts.MaxScenes)
		if err != nil {
			return nil, fmt.Errorf("provenance for %s: %w", c.uri, err)
		}
		if len(scenes) == 0 {
			stats.Massed = append(stats.Massed, c.uri)
			continue
		}
		stats.MemoriesLinked++
		stats.ScenesLinked += len(scenes)
		if opts.DryRun {
			continue
		}
		json, err := db.MarshalStringArray(scenes)
		if err != nil {
			return nil, err
		}
		if _, err := database.ExecContext(ctx,
			`UPDATE memories SET source_scene_uris = ?, updated_at = ? WHERE uri = ?`,
			json, time.Now().UTC().UnixMilli(), c.uri,
		); err != nil {
			return nil, fmt.Errorf("backfill %s: %w", c.uri, err)
		}
	}
	return stats, nil
}

// bestScenes returns the up-to-maxScenes scene URIs whose distilled body
// embedding is cosine-similar (≥ threshold) to the memory embedding, ordered
// most-similar first.
func bestScenes(ctx context.Context, database *sql.DB, memBlob []byte, threshold float64, maxScenes int) ([]string, error) {
	maxDist := 1 - threshold
	rows, err := database.QueryContext(ctx, `
		SELECT 'rmb://scenes/' || lower(id), vec_distance_cosine(embedding, ?) AS d
		FROM scenes
		WHERE embedding IS NOT NULL
		  AND vec_distance_cosine(embedding, ?) <= ?
		ORDER BY d ASC
		LIMIT ?`, memBlob, memBlob, maxDist, maxScenes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uri string
		var d float64
		if err := rows.Scan(&uri, &d); err != nil {
			return nil, err
		}
		out = append(out, uri)
	}
	return out, rows.Err()
}
