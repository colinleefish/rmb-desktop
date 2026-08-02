package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/correction"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
	"github.com/google/uuid"
)

const distillDelay = 1 * time.Second

type MemoryDistiller interface {
	DistillMemory(ctx context.Context, category, slug, atomsJSON string, corrections []string) (string, error)
}

type Worker struct {
	db  *sql.DB
	llm MemoryDistiller
	cfg config.PipelineConfig
	log *slog.Logger
	now func() time.Time
}

func NewWorker(database *sql.DB, llm MemoryDistiller, cfg config.PipelineConfig, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{db: database, llm: llm, cfg: cfg, log: log, now: time.Now}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.llm == nil {
		return fmt.Errorf("l3 worker requires llm client")
	}
	interval := w.cfg.L3PollInterval
	if interval <= 0 {
		return fmt.Errorf("invalid l3 poll interval")
	}
	w.log.Info("l3 memory worker started", "poll_interval", interval)
	w.runOneCycle(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("l3 memory worker stopped")
			return nil
		case <-ticker.C:
			w.runOneCycle(ctx)
		}
	}
}

func (w *Worker) runOneCycle(ctx context.Context) {
	workerlock.GlobalLock.Lock()
	defer workerlock.GlobalLock.Unlock()
	if err := w.rollup(ctx); err != nil {
		w.log.Error("l3 rollup failed", "err", err)
	}
}

func (w *Worker) rollup(ctx context.Context) error {
	pendingIDs, err := w.pendingSessionIDs(ctx)
	if err != nil {
		return err
	}
	if len(pendingIDs) == 0 {
		return nil
	}

	atoms, err := loadAllAtoms(ctx, w.db)
	if err != nil {
		return err
	}

	buckets, skipped := groupAtomsIntoBuckets(atoms)
	if skipped > 0 {
		w.log.Info("l3 skipped slug-less atoms", "count", skipped)
	}
	if len(buckets) == 0 {
		return w.markSessionsIdle(ctx, pendingIDs)
	}

	scenes, err := loadAllScenes(ctx, w.db)
	if err != nil {
		return err
	}
	index := buildAtomSceneIndex(scenes)

	bucketURIs := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		bucketURIs = append(bucketURIs, bucket.URI)
	}
	corrByTarget, err := correction.ForTargets(ctx, w.db, bucketURIs)
	if err != nil {
		return fmt.Errorf("load corrections: %w", err)
	}

	transientPending := false
	for _, bucket := range buckets {
		srcScenes := sourceSceneURIsFor(bucket, index)
		corrStatements, corrURIs := correction.SplitSummaries(corrByTarget[bucket.URI])

		unchanged, err := w.bucketUnchanged(ctx, bucket, srcScenes, corrURIs)
		if err != nil {
			w.log.Warn("l3 provenance check failed", "uri", bucket.URI, "err", err)
			transientPending = true
			break
		}
		if unchanged {
			continue
		}

		pm, err := w.distillBucket(ctx, bucket, corrStatements)
		if err != nil {
			if llm.IsTransientError(err) {
				w.log.Warn("l3 transient error", "uri", bucket.URI, "err", err)
				transientPending = true
				break
			}
			w.log.Warn("l3 bucket failed", "uri", bucket.URI, "err", err)
			continue
		}

		if err := w.persistMemory(ctx, bucket, pm, srcScenes, corrURIs); err != nil {
			if llm.IsTransientError(err) {
				transientPending = true
				break
			}
			w.log.Warn("l3 persist failed", "uri", bucket.URI, "err", err)
			continue
		}
		time.Sleep(distillDelay)
	}

	if transientPending {
		w.log.Info("l3 rollup incomplete, leaving sessions pending", "count", len(pendingIDs))
		return nil
	}
	return w.markSessionsIdle(ctx, pendingIDs)
}

func (w *Worker) pendingSessionIDs(ctx context.Context) ([]string, error) {
	rows, err := w.db.QueryContext(ctx, `SELECT session_id FROM pipeline_state WHERE l3_status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (w *Worker) distillBucket(ctx context.Context, bucket Bucket, corrections []string) (ParsedMemory, error) {
	chunks := chunkAtoms(bucket.Atoms, w.cfg.L3MaxAtoms)
	if len(chunks) == 1 {
		atomsJSON, err := serializeAtomsForLLM(chunks[0])
		if err != nil {
			return ParsedMemory{}, err
		}
		raw, err := w.llm.DistillMemory(ctx, bucket.Category, bucket.Slug, atomsJSON, corrections)
		if err != nil {
			return ParsedMemory{}, err
		}
		return parseDistillResponse(raw)
	}

	partials := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		atomsJSON, err := serializeAtomsForLLM(chunk)
		if err != nil {
			return ParsedMemory{}, err
		}
		raw, err := w.llm.DistillMemory(ctx, bucket.Category, bucket.Slug, atomsJSON, nil)
		if err != nil {
			return ParsedMemory{}, err
		}
		pm, err := parseDistillResponse(raw)
		if err != nil {
			return ParsedMemory{}, err
		}
		partials = append(partials, pm.Body)
		time.Sleep(distillDelay)
	}

	mergedJSON, err := serializePartialsForLLM(partials)
	if err != nil {
		return ParsedMemory{}, err
	}
	raw, err := w.llm.DistillMemory(ctx, bucket.Category, bucket.Slug, mergedJSON, corrections)
	if err != nil {
		return ParsedMemory{}, err
	}
	return parseDistillResponse(raw)
}

func (w *Worker) bucketUnchanged(ctx context.Context, bucket Bucket, srcScenes, corrURIs []string) (bool, error) {
	var sourceScenesJSON, sourceCorrJSON string
	var category string
	err := w.db.QueryRowContext(ctx, `
		SELECT source_scene_uris, source_correction_uris, category FROM memories
		WHERE uri = ? AND superseded_at IS NULL`, bucket.URI,
	).Scan(&sourceScenesJSON, &sourceCorrJSON, &category)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if bucket.Category == model.AtomCategoryEvents {
		return true, nil
	}
	existing, err := db.UnmarshalStringArray(sourceScenesJSON)
	if err != nil {
		return false, err
	}
	existingCorr, err := db.UnmarshalStringArray(sourceCorrJSON)
	if err != nil {
		return false, err
	}
	return equalStringSets(existing, srcScenes) && equalStringSets(existingCorr, corrURIs), nil
}

func (w *Worker) persistMemory(ctx context.Context, bucket Bucket, pm ParsedMemory, sourceSceneURIs, sourceCorrectionURIs []string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	nowMS := w.now().UTC().UnixMilli()
	sceneJSON, err := db.MarshalStringArray(sourceSceneURIs)
	if err != nil {
		return err
	}
	corrJSON, err := db.MarshalStringArray(sourceCorrectionURIs)
	if err != nil {
		return err
	}

	if bucket.Category == model.AtomCategoryEvents {
		var count int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM memories WHERE uri = ? AND superseded_at IS NULL`, bucket.URI,
		).Scan(&count)
		if err != nil {
			return err
		}
		if count > 0 {
			return tx.Commit()
		}
		if err := insertMemory(ctx, tx, bucket, pm, sceneJSON, corrJSON, 1, nowMS); err != nil {
			return err
		}
		return tx.Commit()
	}

	var activeID, activeBody string
	var version int
	err = tx.QueryRowContext(ctx, `
		SELECT id, COALESCE(body, ''), version FROM memories
		WHERE uri = ? AND superseded_at IS NULL`, bucket.URI,
	).Scan(&activeID, &activeBody, &version)
	if err == sql.ErrNoRows {
		if err := insertMemory(ctx, tx, bucket, pm, sceneJSON, corrJSON, 1, nowMS); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("load active memory: %w", err)
	}
	if activeBody == pm.Body {
		return tx.Commit()
	}

	_, err = tx.ExecContext(ctx, `UPDATE memories SET superseded_at = ? WHERE id = ?`, nowMS, activeID)
	if err != nil {
		return fmt.Errorf("supersede memory: %w", err)
	}
	if err := insertMemory(ctx, tx, bucket, pm, sceneJSON, corrJSON, version+1, nowMS); err != nil {
		return err
	}
	return tx.Commit()
}

func insertMemory(ctx context.Context, tx *sql.Tx, bucket Bucket, pm ParsedMemory, sceneJSON, corrJSON string, version int, nowMS int64) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var slugPtr *string
	if bucket.Slug != "" {
		slugPtr = &bucket.Slug
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories (id, uri, category, slug, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), bucket.URI, bucket.Category, slugPtr, version, pm.Abstract, pm.Body, sceneJSON, corrJSON, nowMS, nowMS,
	)
	if err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`,
		id.String(), pm.Abstract, pm.Body,
	)
	if err != nil {
		return fmt.Errorf("index memory fts: %w", err)
	}
	return nil
}

func (w *Worker) markSessionsIdle(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	nowMS := w.now().UTC().UnixMilli()
	for _, id := range ids {
		_, err := w.db.ExecContext(ctx, `
			UPDATE pipeline_state SET l3_status = 'idle', l3_advanced_at = ?, updated_at = ?
			WHERE session_id = ? AND l3_status = 'pending'`, nowMS, nowMS, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadAllAtoms(ctx context.Context, database *sql.DB) ([]model.Atom, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at
		FROM atoms ORDER BY category ASC, created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAtoms(rows)
}

func loadAllScenes(ctx context.Context, database *sql.DB) ([]model.Scene, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, session_id, display_name, abstract, body, source_atoms, created_at, updated_at FROM scenes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Scene
	for rows.Next() {
		var s model.Scene
		var displayName, abstract, body sql.NullString
		var sourceJSON string
		if err := rows.Scan(&s.ID, &s.SessionID, &displayName, &abstract, &body, &sourceJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if displayName.Valid {
			s.DisplayName = &displayName.String
		}
		if abstract.Valid {
			s.Abstract = &abstract.String
		}
		if body.Valid {
			s.Body = &body.String
		}
		s.SourceAtoms, err = db.UnmarshalStringArray(sourceJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanAtoms(rows *sql.Rows) ([]model.Atom, error) {
	var out []model.Atom
	for rows.Next() {
		var a model.Atom
		var sceneName, slug sql.NullString
		var sourceJSON string
		if err := rows.Scan(&a.ID, &a.SessionID, &a.Category, &a.Priority, &sceneName, &slug, &a.Content, &sourceJSON, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if sceneName.Valid {
			a.SceneName = &sceneName.String
		}
		if slug.Valid {
			a.Slug = &slug.String
		}
		var err error
		a.SourceTurnIDs, err = db.UnmarshalStringArray(sourceJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
