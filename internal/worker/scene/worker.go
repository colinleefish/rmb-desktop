package scene

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/pipeline"
	"github.com/colinleefish/rmb-desktop/internal/worker/backpressure"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
)

type SceneBuilder interface {
	BuildScenes(ctx context.Context, atomsJSON string) (string, error)
	SummarizeSessionAbstract(ctx context.Context, sceneAbstracts string) (string, error)
}

type Worker struct {
	db    *sql.DB
	llm   SceneBuilder
	cfg   config.PipelineConfig
	locks *workerlock.SessionLocks
	log   *slog.Logger
	now   func() time.Time
	bp    *backpressure.Controller
	reg   *debug.Registry
}

func NewWorker(database *sql.DB, llm SceneBuilder, cfg config.PipelineConfig, locks *workerlock.SessionLocks, log *slog.Logger, reg *debug.Registry) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{
		db:    database,
		llm:   llm,
		cfg:   cfg,
		locks: locks,
		log:   log,
		now:   time.Now,
		bp:    backpressure.New(cfg.L2MinConcurrency, cfg.L2MaxConcurrency),
		reg:   reg,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.llm == nil {
		return fmt.Errorf("l2 worker requires llm client")
	}
	interval := w.cfg.L2PollInterval
	if interval <= 0 {
		return fmt.Errorf("invalid l2 poll interval")
	}
	w.reg.WorkerStarted("l2")
	defer w.reg.WorkerStopped("l2")
	w.log.Info("l2 scene worker started",
		"poll_interval", interval,
		"min_concurrency", w.bp.Min(),
		"max_concurrency", w.bp.Max(),
	)
	w.runOneCycle(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("l2 scene worker stopped")
			return nil
		case <-ticker.C:
			w.runOneCycle(ctx)
		}
	}
}

func (w *Worker) runOneCycle(ctx context.Context) {
	endCycle := w.reg.BeginCycle("l2")
	defer endCycle(nil)

	ids, err := w.selectCandidateSessions(ctx)
	if err != nil {
		w.log.Error("l2 select candidates failed", "err", err)
		return
	}
	if len(ids) == 0 {
		w.bp.EndCycle(0)
		return
	}

	if n, err := w.countPendingSessions(ctx); err == nil {
		if prev, next := w.bp.SeedFromBacklog(n); prev != next {
			w.log.Info("l2 concurrency seeded", "from", prev, "to", next, "pending", n)
		}
	}

	limit := w.bp.Limit()
	w.reg.SetConcurrency("l2", limit)
	w.log.Info("l2 cycle", "candidates", len(ids), "concurrency", limit)

	remaining := ids
	for len(remaining) > 0 {
		if ctx.Err() != nil {
			return
		}
		limit = w.bp.Limit()
		n := limit
		if n > len(remaining) {
			n = len(remaining)
		}
		batch := remaining[:n]
		remaining = remaining[n:]

		backpressure.RunParallel(ctx, batch, limit, func(ctx context.Context, id string) {
			err := w.processSession(ctx, id)
			w.bp.Observe(backpressure.Outcome{
				Err:      err,
				Pressure: llm.IsTransientError(err),
			})
			if err != nil && !llm.IsTransientError(err) {
				w.log.Error("l2 process session failed", "session_id", id, "err", err)
			}
		})

		pendingHint := len(remaining)
		if n, err := w.countPendingSessions(ctx); err == nil {
			pendingHint = n
		}
		prev, next := w.bp.EndCycle(pendingHint)
		w.reg.SetBackpressure("l2", w.bp.Min(), w.bp.Max(), next, pendingHint)
		if prev != next {
			w.log.Info("l2 concurrency adjusted", "from", prev, "to", next, "pending", pendingHint)
		}
		if next < prev {
			return
		}
	}
}

func (w *Worker) countPendingSessions(ctx context.Context) (int, error) {
	var n int
	err := w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pipeline_state
		WHERE l2_status IN ('pending', 'failed') AND l1_status != 'running'`).Scan(&n)
	return n, err
}

func (w *Worker) selectCandidateSessions(ctx context.Context) ([]string, error) {
	limit := w.bp.Max()
	if limit < 1 {
		limit = 4
	}
	fetch := limit * 2
	rows, err := w.db.QueryContext(ctx, `
		SELECT session_id FROM pipeline_state
		WHERE l2_status IN ('pending', 'failed') AND l1_status != 'running'
		ORDER BY updated_at LIMIT ?`, fetch)
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

type sceneBatch struct {
	SessionKey string
	SessionID  string
	Atoms      []model.Atom
}

func (w *Worker) processSession(ctx context.Context, sessionID string) error {
	unlock := w.locks.Lock(sessionID)
	defer unlock()

	w.reg.BeginSession(sessionID, "", "l2", "prepare")
	batch, err := w.prepareBatch(ctx, sessionID)
	if err != nil || batch == nil {
		w.reg.EndSession(sessionID, "l2")
		return err
	}
	defer w.reg.EndSession(sessionID, "l2")

	groups := groupAtomsBySceneName(batch.Atoms)
	chunks := chunkGroups(groups, w.cfg.L2MaxAtoms, w.cfg.L2MaxScenes)
	validURIs := atomURISet(batch.Atoms)

	var parsed []ParsedScene
	for _, chunk := range chunks {
		atomsJSON, err := serializeAtomsForLLM(chunk)
		if err != nil {
			return w.handleProcessError(ctx, sessionID, err)
		}
		w.reg.BeginSession(sessionID, batch.SessionKey, "l2", "llm.build_scenes")
		raw, err := w.llm.BuildScenes(ctx, atomsJSON)
		if err != nil {
			return w.handleProcessError(ctx, sessionID, fmt.Errorf("llm build scenes: %w", err))
		}
		w.reg.BeginSession(sessionID, batch.SessionKey, "l2", "parse.build_scenes")
		chunkScenes, err := parseBuildScenesResponse(raw, validURIs)
		if err != nil {
			return w.handleProcessError(ctx, sessionID, fmt.Errorf("parse build scenes: %w", err))
		}
		parsed = append(parsed, chunkScenes...)
	}

	w.reg.BeginSession(sessionID, batch.SessionKey, "l2", "llm.session_abstract")
	abstract, err := w.llm.SummarizeSessionAbstract(ctx, joinSceneAbstracts(parsed))
	if err != nil {
		return w.handleProcessError(ctx, sessionID, fmt.Errorf("llm session abstract: %w", err))
	}

	w.reg.BeginSession(sessionID, batch.SessionKey, "l2", "persist.scenes")
	return w.persistScenes(ctx, batch, parsed, abstract)
}

func (w *Worker) prepareBatch(ctx context.Context, sessionID string) (*sceneBatch, error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var sessionKey string
	err = tx.QueryRowContext(ctx, `SELECT session_key FROM sessions WHERE id = ?`, sessionID).Scan(&sessionKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var l1Status, l2Status string
	var l1AdvancedAt sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT l1_status, l2_status, l1_advanced_at FROM pipeline_state WHERE session_id = ?`, sessionID,
	).Scan(&l1Status, &l2Status, &l1AdvancedAt)
	if err != nil {
		return nil, fmt.Errorf("load pipeline_state: %w", err)
	}

	var l1Adv *int64
	if l1AdvancedAt.Valid {
		v := l1AdvancedAt.Int64
		l1Adv = &v
	}
	if !shouldRunL2(w.now().UTC(), l1Status, l2Status, l1Adv, w.cfg.L2DelayAfterL1) {
		return nil, nil
	}

	atoms, err := loadSessionAtoms(ctx, tx, sessionID)
	if err != nil {
		return nil, err
	}
	if len(atoms) == 0 {
		_, _ = tx.ExecContext(ctx, `UPDATE pipeline_state SET l2_status = 'idle' WHERE session_id = ?`, sessionID)
		return nil, tx.Commit()
	}

	nowMS := w.now().UTC().UnixMilli()
	_, err = tx.ExecContext(ctx, `
		UPDATE pipeline_state SET
			l2_status = 'running',
			l2_started_at = ?,
			l2_last_error = NULL,
			updated_at = ?
		WHERE session_id = ?`, nowMS, nowMS, sessionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &sceneBatch{SessionKey: sessionKey, SessionID: sessionID, Atoms: atoms}, nil
}

func loadSessionAtoms(ctx context.Context, tx *sql.Tx, sessionID string) ([]model.Atom, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at
		FROM atoms WHERE session_id = ? ORDER BY created_at ASC, id ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
		a.SourceTurnIDs, err = db.UnmarshalStringArray(sourceJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (w *Worker) persistScenes(ctx context.Context, batch *sceneBatch, scenes []ParsedScene, sessionAbstract string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	nowMS := w.now().UTC().UnixMilli()
	dupCount := make(map[string]int, len(scenes))
	keepIDs := make([]string, 0, len(scenes))

	for _, s := range scenes {
		dupCount[s.DisplayName]++
		sceneID := sceneIDForName(batch.SessionID, s.DisplayName, dupCount[s.DisplayName])
		keepIDs = append(keepIDs, sceneID)

		sourceJSON, err := db.MarshalStringArray(s.SourceAtoms)
		if err != nil {
			return err
		}

		var existingCreated int64
		err = tx.QueryRowContext(ctx, `SELECT created_at FROM scenes WHERE id = ?`, sceneID).Scan(&existingCreated)
		createdAt := nowMS
		if err == nil {
			createdAt = existingCreated
		} else if err != sql.ErrNoRows {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				display_name = excluded.display_name,
				abstract = excluded.abstract,
				body = excluded.body,
				source_atoms = excluded.source_atoms,
				embedding = NULL,
				updated_at = excluded.updated_at`,
			sceneID, batch.SessionID, s.DisplayName, s.Abstract, s.Body, sourceJSON, createdAt, nowMS,
		)
		if err != nil {
			return fmt.Errorf("upsert scene: %w", err)
		}

		var sceneRowID int64
		if err := tx.QueryRowContext(ctx, `SELECT rowid FROM scenes WHERE id = ?`, sceneID).Scan(&sceneRowID); err != nil {
			return fmt.Errorf("scene rowid: %w", err)
		}

		// FTS5 external-content tables: use the documented delete command instead of DELETE FROM.
		if _, err = tx.ExecContext(ctx, `INSERT INTO scenes_fts(scenes_fts, rowid) VALUES('delete', ?)`, sceneRowID); err != nil {
			return fmt.Errorf("clear scene fts: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO scenes_fts(rowid, abstract, body) VALUES (?, ?, ?)`,
			sceneRowID, s.Abstract, s.Body,
		)
		if err != nil {
			return fmt.Errorf("index scene fts: %w", err)
		}
	}

	// Prune stale scenes for this session.
	if len(keepIDs) > 0 {
		placeholders := make([]string, len(keepIDs))
		args := make([]any, 0, len(keepIDs)+1)
		args = append(args, batch.SessionID)
		for i, id := range keepIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query := fmt.Sprintf(`DELETE FROM scenes WHERE session_id = ? AND id NOT IN (%s)`, joinPlaceholders(placeholders))
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("prune scenes: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM scenes WHERE session_id = ?`, batch.SessionID); err != nil {
			return fmt.Errorf("prune scenes: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx, `UPDATE sessions SET abstract = ?, updated_at = ? WHERE id = ?`,
		sessionAbstract, nowMS, batch.SessionID)
	if err != nil {
		return fmt.Errorf("update session abstract: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE pipeline_state SET
			l2_status = 'idle',
			l2_advanced_at = ?,
			l2_started_at = NULL,
			l2_last_error = NULL,
			l3_status = 'pending',
			updated_at = ?
		WHERE session_id = ?`, nowMS, nowMS, batch.SessionID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	w.log.Info("l2 built scenes", "session", batch.SessionKey, "count", len(scenes))
	return nil
}

func (w *Worker) handleProcessError(ctx context.Context, sessionID string, cause error) error {
	if llm.IsTransientError(cause) {
		w.log.Warn("l2 transient error", "session_id", sessionID, "err", cause)
		_ = pipeline.MarkPending(ctx, w.db, sessionID, pipeline.StageL2, cause.Error(), w.now())
		return cause
	}
	_ = pipeline.MarkFailed(ctx, w.db, sessionID, pipeline.StageL2, cause.Error(), w.now())
	return cause
}

func joinPlaceholders(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}
