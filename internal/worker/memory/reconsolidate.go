package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

// ReconsolidateResult reports one evidence-grounded re-distillation
// (issue #33 / plan §9.3e): an eroded chain (e.g. 131 versions of drift)
// rebuilt statically from its atom evidence.
type ReconsolidateResult struct {
	URI           string `json:"uri"`
	OldVersion    int    `json:"old_version"`
	NewVersion    int    `json:"new_version"`
	OldBodyChars  int    `json:"old_body_chars"`
	NewBodyChars  int    `json:"new_body_chars"`
	BucketAtoms   int    `json:"bucket_atoms"`
	AtomIDsHashed string `json:"source_atom_hash"`
	OccurredAtMS  int64  `json:"occurred_at_ms,omitempty"`
	NewAbstract   string `json:"new_abstract"`
	NewBody       string `json:"new_body"`
}

// Reconsolidate rebuilds a memory from its ATOM evidence, deliberately
// bypassing the consolidation gates: the whole point (§9.3e) is to replace
// an eroded rewrite chain with a fresh, evidence-grounded distill. The new
// body supersedes the active row and bumps the version.
//
// The memory uri must be rmb://<category>/<slug> with category in
// events/entities/preferences (profile singleton: rmb://profile).
func Reconsolidate(ctx context.Context, database *sql.DB, distiller MemoryDistiller, cfg config.PipelineConfig, log *slog.Logger, rawURI string) (*ReconsolidateResult, error) {
	u, err := uri.Parse(strings.TrimSpace(rawURI))
	if err != nil {
		return nil, err
	}
	category, slug := u.Scope, ""
	if category == model.AtomCategoryProfile {
		slug = ""
	} else if len(u.Segments) > 0 {
		slug = u.Segments[0]
	}
	switch category {
	case model.AtomCategoryEvents, model.AtomCategoryEntities, model.AtomCategoryPreferences, model.AtomCategoryProfile:
	default:
		return nil, fmt.Errorf("reconsolidate supports memories (events/entities/preferences/profile), got %q", rawURI)
	}

	// Load the evidence: all atoms for this subject.
	rows, err := database.QueryContext(ctx, `
		SELECT id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at
		FROM atoms WHERE category = ? ORDER BY created_at ASC, id ASC`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all, bucket []model.Atom
	for rows.Next() {
		var a model.Atom
		var sceneName, slugPtr sql.NullString
		var sourceJSON string
		if err := rows.Scan(&a.ID, &a.SessionID, &a.Category, &a.Priority, &sceneName, &slugPtr, &a.Content, &sourceJSON, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if sceneName.Valid {
			s := sceneName.String
			a.SceneName = &s
		}
		if slugPtr.Valid && slugPtr.String != "" {
			s := slugPtr.String
			a.Slug = &s
		}
		all = append(all, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if category == model.AtomCategoryProfile {
		bucket = all
	} else {
		for _, a := range all {
			if a.Slug != nil && *a.Slug == slug {
				bucket = append(bucket, a)
			}
		}
	}
	if len(bucket) == 0 {
		return nil, fmt.Errorf("no atoms found for %s — nothing to reconsolidate from", rawURI)
	}

	w := NewWorker(database, distiller, nil, cfg, log, nil)
	b := Bucket{Category: category, Slug: slug, URI: u.String(), Atoms: bucket}

	// Current state (may not exist yet — e.g. deferred subject; then this is
	// effectively a forced graduation from evidence).
	var activeID string
	var oldVersion int
	var oldBody string
	err = database.QueryRowContext(ctx, `
		SELECT id, version, COALESCE(body,'') FROM memories WHERE uri = ? AND superseded_at IS NULL`, b.URI,
	).Scan(&activeID, &oldVersion, &oldBody)
	hasActive := err == nil
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Fresh distill from evidence (related-events + reduce path apply).
	pm, err := w.distillBucket(ctx, b, nil)
	if err != nil {
		return nil, fmt.Errorf("reconsolidate distill: %w", err)
	}
	pm.Body = capBody(pm.Body)

	nowMS := time.Now().UTC().UnixMilli()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	newVersion := oldVersion + 1
	occurred := int64(0)
	if hasActive {
		if _, err := tx.ExecContext(ctx, `UPDATE memories SET superseded_at = ? WHERE id = ?`, nowMS, activeID); err != nil {
			return nil, err
		}
	}
	if category == model.AtomCategoryEvents {
		occurred = eventOccurredAt(b.Slug, pm, nowMS)
	}

	sceneJSON := "[]"
	corrJSON := "[]"
	atomHash := hashAtomIDs(b.Atoms)
	if hasActive {
		// Preserve the subject's accumulated provenance.
		var sj, cj string
		if err := database.QueryRowContext(ctx, `SELECT source_scene_uris, source_correction_uris FROM memories WHERE id = ?`, activeID).Scan(&sj, &cj); err == nil {
			sceneJSON, corrJSON = sj, cj
		}
	}
	if err := insertMemory(ctx, tx, b, pm, sceneJSON, corrJSON, atomHash, newVersion, nowMS, occurred); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &ReconsolidateResult{
		URI:           b.URI,
		OldVersion:    oldVersion,
		NewVersion:    newVersion,
		OldBodyChars:  len([]rune(oldBody)),
		NewBodyChars:  len([]rune(pm.Body)),
		BucketAtoms:   len(bucket),
		AtomIDsHashed: atomHash,
		OccurredAtMS:  occurred,
		NewAbstract:   pm.Abstract,
		NewBody:       pm.Body,
	}, nil
}
