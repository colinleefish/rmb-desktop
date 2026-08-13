package extract

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/uri"
	"github.com/colinleefish/rmb-desktop/internal/worker/backpressure"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
	"github.com/google/uuid"
)

type AtomExtractor interface {
	ExtractAtoms(ctx context.Context, messagesJSONL string) (string, error)
}

type Worker struct {
	db     *sql.DB
	llm    AtomExtractor
	cfg    config.PipelineConfig
	locks  *workerlock.SessionLocks
	log    *slog.Logger
	now    func() time.Time
	bp     *backpressure.Controller
}

func NewWorker(database *sql.DB, llm AtomExtractor, cfg config.PipelineConfig, locks *workerlock.SessionLocks, log *slog.Logger) *Worker {
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
		bp:    backpressure.New(cfg.L1MinConcurrency, cfg.L1MaxConcurrency),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.llm == nil {
		return fmt.Errorf("l1 worker requires llm client")
	}
	interval := w.cfg.L1PollInterval
	if interval <= 0 {
		return fmt.Errorf("invalid l1 poll interval")
	}
	w.log.Info("l1 extract worker started",
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
			w.log.Info("l1 extract worker stopped")
			return nil
		case <-ticker.C:
			w.runOneCycle(ctx)
		}
	}
}

func (w *Worker) runOneCycle(ctx context.Context) {
	ids, err := w.selectCandidateSessions(ctx)
	if err != nil {
		w.log.Error("l1 select candidates failed", "err", err)
		return
	}
	if len(ids) == 0 {
		w.bp.EndCycle(0)
		return
	}

	if n, err := w.countPendingSessions(ctx); err == nil {
		if prev, next := w.bp.SeedFromBacklog(n); prev != next {
			w.log.Info("l1 concurrency seeded", "from", prev, "to", next, "pending", n)
		}
	}

	limit := w.bp.Limit()
	w.log.Info("l1 cycle", "candidates", len(ids), "concurrency", limit)

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
				w.log.Error("l1 process session failed", "session_id", id, "err", err)
			}
		})

		pendingHint := len(remaining)
		if n, err := w.countPendingSessions(ctx); err == nil {
			pendingHint = n
		}
		prev, next := w.bp.EndCycle(pendingHint)
		if prev != next {
			w.log.Info("l1 concurrency adjusted", "from", prev, "to", next, "pending", pendingHint)
		}
		// Stop this cycle on pressure so we don't keep hammering the LLM.
		if next < prev {
			return
		}
	}
}

func (w *Worker) countPendingSessions(ctx context.Context) (int, error) {
	var n int
	err := w.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT session_id)
		FROM session_turns
		WHERE l1_extracted_at IS NULL`).Scan(&n)
	return n, err
}

func (w *Worker) selectCandidateSessions(ctx context.Context) ([]string, error) {
	limit := w.bp.Max()
	if limit < 1 {
		limit = 8
	}
	// Fetch a bit ahead of max concurrency so scaled-up cycles stay fed.
	fetch := limit * 2
	// FIFO by oldest unextracted turn: otherwise ORDER BY session_id puts
	// late-UUID (backfilled) sessions at the tail and, combined with the
	// backpressure early-exit below, they starve forever.
	rows, err := w.db.QueryContext(ctx, `
		SELECT session_id
		FROM session_turns
		WHERE l1_extracted_at IS NULL
		GROUP BY session_id
		ORDER BY MIN(created_at) ASC
		LIMIT ?`, fetch)
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

type extractBatch struct {
	SessionKey      string
	SessionID       string
	Turns           []model.SessionTurn
	MessagesJSONL   string
	WarmupThreshold int
}

func (w *Worker) processSession(ctx context.Context, sessionID string) error {
	unlock := w.locks.Lock(sessionID)
	defer unlock()

	batch, err := w.prepareBatch(ctx, sessionID)
	if err != nil || batch == nil {
		return err
	}

	raw, err := w.llm.ExtractAtoms(ctx, batch.MessagesJSONL)
	if err != nil {
		return w.handleProcessError(ctx, sessionID, fmt.Errorf("llm extract: %w", err))
	}

	parsed, err := parseExtractResponse(raw)
	if err != nil {
		return w.handleProcessError(ctx, sessionID, fmt.Errorf("parse extract response: %w", err))
	}

	return w.persistBatch(ctx, sessionID, batch, parsed)
}

func (w *Worker) prepareBatch(ctx context.Context, sessionID string) (*extractBatch, error) {
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
		return nil, fmt.Errorf("load session: %w", err)
	}

	var (
		l1Status             string
		l1TurnsSinceAdvanced int
		warmupThreshold      int
	)
	err = tx.QueryRowContext(ctx, `
		SELECT l1_status, l1_turns_since_advanced, warmup_threshold
		FROM pipeline_state WHERE session_id = ?`, sessionID,
	).Scan(&l1Status, &l1TurnsSinceAdvanced, &warmupThreshold)
	if err == sql.ErrNoRows {
		nowMS := w.now().UTC().UnixMilli()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, warmup_threshold, updated_at)
			VALUES (?, 'idle', 'idle', 'idle', 2, ?)`, sessionID, nowMS)
		if err != nil {
			return nil, err
		}
		l1Status = model.PipelineStatusIdle
		warmupThreshold = 2
	} else if err != nil {
		return nil, fmt.Errorf("load pipeline_state: %w", err)
	}

	maxTurns := w.cfg.L1MaxTurns
	if maxTurns <= 0 {
		maxTurns = 8
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, session_id, messages_json, created_at
		FROM session_turns
		WHERE session_id = ? AND l1_extracted_at IS NULL
		ORDER BY created_at ASC
		LIMIT ?`, sessionID, maxTurns)
	if err != nil {
		return nil, err
	}
	var turns []model.SessionTurn
	for rows.Next() {
		var t model.SessionTurn
		if err := rows.Scan(&t.ID, &t.SessionID, &t.MessagesJSON, &t.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		turns = append(turns, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(turns) == 0 {
		if l1Status == model.PipelineStatusPending {
			_, _ = tx.ExecContext(ctx, `
				UPDATE pipeline_state SET l1_status = 'idle', l1_turns_since_advanced = 0 WHERE session_id = ?`,
				sessionID)
			return nil, tx.Commit()
		}
		return nil, nil
	}

	lastTurnAt := time.UnixMilli(turns[len(turns)-1].CreatedAt).UTC()
	if !shouldRunL1(
		w.now().UTC(), l1Status, len(turns), l1TurnsSinceAdvanced, warmupThreshold,
		w.cfg.L1EveryN, w.cfg.L1Warmup, w.cfg.L1IdleSeconds, lastTurnAt,
	) {
		return nil, nil
	}

	_, err = tx.ExecContext(ctx, `UPDATE pipeline_state SET l1_status = 'running' WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &extractBatch{
		SessionKey:      sessionKey,
		SessionID:       sessionID,
		Turns:           turns,
		MessagesJSONL:   mergeTurnMessages(turns, w.cfg.L1MaxChars),
		WarmupThreshold: warmupThreshold,
	}, nil
}

func (w *Worker) persistBatch(ctx context.Context, sessionID string, batch *extractBatch, parsed []llmAtom) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	nowMS := w.now().UTC().UnixMilli()
	turnIndex := buildTurnIndex(batch.Turns)

	for _, a := range parsed {
		atomID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate atom id: %w", err)
		}
		sourceIDs, err := resolveSourceTurnIDs(a.SourceTurnIndices, turnIndex)
		if err != nil {
			w.log.Warn("atom source fallback", "session", batch.SessionKey, "err", err)
			sourceIDs = []string{batch.Turns[0].ID}
		}

		var slugPtr *string
		if a.Slug != "" && a.Category != model.AtomCategoryProfile {
			if sanitized, err := uri.SanitizeSlug(a.Slug); err == nil {
				slugPtr = &sanitized
			}
		}
		var scenePtr *string
		if sceneName := strings.TrimSpace(a.SceneName); sceneName != "" {
			scenePtr = &sceneName
		}
		priority := a.Priority
		if priority == 0 {
			priority = 50
		}

		sourceJSON, err := db.MarshalStringArray(sourceIDs)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO atoms (id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			atomID.String(), sessionID, a.Category, priority, scenePtr, slugPtr, a.Content, sourceJSON, nowMS, nowMS,
		)
		if err != nil {
			return fmt.Errorf("insert atom: %w", err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO atoms_fts(rowid, content) VALUES ((SELECT rowid FROM atoms WHERE id = ?), ?)`,
			atomID.String(), a.Content,
		)
		if err != nil {
			return fmt.Errorf("index atom fts: %w", err)
		}
	}

	for _, t := range batch.Turns {
		_, err = tx.ExecContext(ctx, `
			UPDATE session_turns SET l1_extracted_at = ? WHERE id = ? AND l1_extracted_at IS NULL`,
			nowMS, t.ID,
		)
		if err != nil {
			return fmt.Errorf("mark turn extracted: %w", err)
		}
	}

	nextWarmup := nextWarmupThreshold(batch.WarmupThreshold, w.cfg.L1EveryN, w.cfg.L1Warmup)
	_, err = tx.ExecContext(ctx, `
		UPDATE pipeline_state SET
			l1_status = 'idle',
			l1_advanced_at = ?,
			l1_turns_since_advanced = 0,
			warmup_threshold = ?,
			l2_status = 'pending',
			updated_at = ?
		WHERE session_id = ?`,
		nowMS, nextWarmup, nowMS, sessionID,
	)
	if err != nil {
		return fmt.Errorf("update pipeline_state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	w.log.Info("l1 extracted", "session", batch.SessionKey, "turns", len(batch.Turns), "atoms", len(parsed))
	return nil
}

func (w *Worker) handleProcessError(ctx context.Context, sessionID string, cause error) error {
	if llm.IsTransientError(cause) {
		w.log.Warn("l1 transient error", "session_id", sessionID, "err", cause)
		_, _ = w.db.ExecContext(ctx, `UPDATE pipeline_state SET l1_status = 'pending' WHERE session_id = ?`, sessionID)
		return cause
	}
	_, _ = w.db.ExecContext(ctx, `UPDATE pipeline_state SET l1_status = 'failed' WHERE session_id = ?`, sessionID)
	return cause
}

func mergeTurnMessages(turns []model.SessionTurn, maxChars int) string {
	var out strings.Builder
	for _, turn := range turns {
		chunk := strings.TrimSpace(turn.MessagesJSON)
		if chunk == "" {
			continue
		}
		next := chunk
		if out.Len() > 0 {
			next = "\n" + next
		}
		if maxChars > 0 && out.Len()+len(next) > maxChars {
			remaining := maxChars - out.Len()
			if remaining <= 0 {
				break
			}
			out.WriteString(next[:remaining])
			break
		}
		out.WriteString(next)
	}
	if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
		out.WriteString("\n")
	}
	return out.String()
}

func buildTurnIndex(turns []model.SessionTurn) map[int]string {
	idx := make(map[int]string, len(turns))
	for i, t := range turns {
		idx[i] = t.ID
	}
	return idx
}

func resolveSourceTurnIDs(indices []int, turnIndex map[int]string) ([]string, error) {
	if len(indices) == 0 {
		return nil, fmt.Errorf("no source_turn_indices")
	}
	out := make([]string, 0, len(indices))
	seen := make(map[string]struct{})
	for _, i := range indices {
		id, ok := turnIndex[i]
		if !ok && i > 0 {
			id, ok = turnIndex[i-1]
		}
		if !ok {
			return nil, fmt.Errorf("invalid turn index %d", i)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
