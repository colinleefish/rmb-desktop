package extract

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/pipeline"
	"github.com/colinleefish/rmb-desktop/internal/recall"
	"github.com/colinleefish/rmb-desktop/internal/textsim"
	"github.com/colinleefish/rmb-desktop/internal/uri"
	"github.com/colinleefish/rmb-desktop/internal/worker/backpressure"
	"github.com/colinleefish/rmb-desktop/internal/worker/shared"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
	"github.com/google/uuid"
)

type AtomExtractor interface {
	ExtractAtoms(ctx context.Context, messagesJSONL string, candidates []llm.SlugCandidate) (string, error)
}

// atomDedupJaccard is the token-Jaccard bar at which a freshly extracted
// atom is treated as a near-verbatim restatement of an existing atom with the
// SAME (category, slug) — or another profile atom — and skipped at ingest
// (issue #27 task 4). Token-set similarity catches restatements, not free
// paraphrases (a reworded sentence drops to ~0.4). Measured on the live
// store (read-only, 2026-08-23): same-subject atom pairs n=1120 — median
// 0.15, p90 0.36, only 1% >= 0.72 (the verbatim repeats that fed the
// audit's redundant-bullet clusters); random same-category pairs n=1200 —
// max 0.43, ZERO >= 0.72. The bar sits inside the empty band 0.43–0.72+,
// so suppression drops only restatements, never new evidence. Fixtures in
// consolidation_test.go.
const atomDedupJaccard = 0.72

// slugCandidatesPerCategory caps how many existing subject slugs per
// category are injected into the extract prompt (P2.1 retrieve-then-
// canonicalize).
const slugCandidatesPerCategory = 6

type Worker struct {
	db    *sql.DB
	llm   AtomExtractor
	cfg   config.PipelineConfig
	locks *workerlock.SessionLocks
	log   *slog.Logger
	now   func() time.Time
	bp    *backpressure.Controller
	reg   *debug.Registry
}

func NewWorker(database *sql.DB, llm AtomExtractor, cfg config.PipelineConfig, locks *workerlock.SessionLocks, log *slog.Logger, reg *debug.Registry) *Worker {
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
		reg:   reg,
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
	shared.RunPoll(ctx, shared.PollOptions{
		Name:       "l1",
		Label:      "l1 extract",
		Interval:   interval,
		Registry:   w.reg,
		Log:        w.log,
		StartAttrs: []any{"min_concurrency", w.bp.Min(), "max_concurrency", w.bp.Max()},
		Cycle:      w.runOneCycle,
	})
	return nil
}

func (w *Worker) runOneCycle(ctx context.Context) {
	shared.RunBackpressuredCycle(ctx, "l1", w.bp, w.reg, w.log, shared.CycleDeps{
		SelectCandidates: w.selectCandidateSessions,
		CountPending:     w.countPendingSessions,
		ProcessSession:   w.processSession,
	})
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
	// Candidates carries existing subject slugs (per category) retrieved for
	// this batch so the extract prompt can canonicalize slugs and persistBatch
	// can deterministically remap variant spellings (P2.1 / issue #27).
	Candidates []llm.SlugCandidate
}

func (w *Worker) processSession(ctx context.Context, sessionID string) error {
	unlock := w.locks.Lock(sessionID)
	defer unlock()

	w.reg.BeginSession(sessionID, "", "l1", "prepare")
	batch, err := w.prepareBatch(ctx, sessionID)
	if err != nil || batch == nil {
		w.reg.EndSession(sessionID, "l1")
		return err
	}

	w.reg.BeginSession(sessionID, batch.SessionKey, "l1", "llm.extract_atoms")
	defer w.reg.EndSession(sessionID, "l1")

	// Retrieve-then-canonicalize (P2.1): existing subject slugs for this
	// batch, so the same subject reuses one slug across sessions. Retrieval
	// failure is non-fatal — extract without candidates.
	batch.Candidates = w.candidateSlugs(ctx, batch.MessagesJSONL)

	raw, err := w.llm.ExtractAtoms(ctx, batch.MessagesJSONL, batch.Candidates)
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
		w.cfg.L1EveryN, w.cfg.L1Warmup, w.cfg.L1Idle, lastTurnAt,
	) {
		return nil, nil
	}

	nowMS := w.now().UTC().UnixMilli()
	_, err = tx.ExecContext(ctx, `
		UPDATE pipeline_state SET
			l1_status = 'running',
			l1_started_at = ?,
			l1_last_error = NULL,
			updated_at = ?
		WHERE session_id = ?`, nowMS, nowMS, sessionID)
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

	// Canonicalize slugs + enforce the event date prefix (P2.1 / issue #27
	// task 2), then drop paraphrase atoms against existing same-subject
	// atoms and within the batch itself (task 4).
	parsed = canonicalizeAtoms(parsed, batch.Candidates, batch.Turns)
	parsed, deduped := w.dedupAtoms(ctx, parsed)

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
			l1_started_at = NULL,
			l1_last_error = NULL,
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
	w.log.Info("l1 extracted", "session", batch.SessionKey, "turns", len(batch.Turns),
		"atoms", len(parsed), "deduped_paraphrases", deduped)
	return nil
}

// canonicalizeAtoms applies two deterministic P2.1 normalizations the LLM may
// miss: (a) event slugs without a YYYY-MM-DD- prefix get the session date
// prepended; (b) a slug that normalizes equal to a retrieved candidate slug
// of the same category is rewritten to the incumbent spelling, so variant
// spellings ("doc-language" vs "docs-language") consolidate into one bucket.
func canonicalizeAtoms(parsed []llmAtom, candidates []llm.SlugCandidate, turns []model.SessionTurn) []llmAtom {
	if len(parsed) == 0 {
		return parsed
	}
	sessionDate := ""
	if len(turns) > 0 {
		sessionDate = time.UnixMilli(turns[len(turns)-1].CreatedAt).Format("2006-01-02")
	}
	eventDatePrefix := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)
	for i := range parsed {
		if parsed[i].Slug == "" || parsed[i].Category == model.AtomCategoryProfile {
			continue
		}
		if parsed[i].Category == model.AtomCategoryEvents && sessionDate != "" &&
			!eventDatePrefix.MatchString(parsed[i].Slug) {
			parsed[i].Slug = sessionDate + "-" + parsed[i].Slug
		}
		for _, c := range candidates {
			if c.Category != parsed[i].Category || c.Slug == parsed[i].Slug {
				continue
			}
			if textsim.SlugEqual(parsed[i].Slug, c.Slug) {
				parsed[i].Slug = c.Slug
				break
			}
		}
	}
	return parsed
}

// dedupAtoms suppresses paraphrase atoms (P2.1 task 4): a new atom whose
// content is token-Jaccard-similar to an existing atom with the same
// (category, slug) — or to another profile atom — is dropped at ingest, as
// are duplicates within the batch itself. Suppression is conservative: only
// same-subject (or profile) comparisons are made, so a similar sentence about
// a different subject always survives.
func (w *Worker) dedupAtoms(ctx context.Context, parsed []llmAtom) ([]llmAtom, int) {
	if len(parsed) == 0 {
		return parsed, 0
	}
	// existing content per compared key
	type key struct{ category, slug string }
	existing := make(map[key][]string)
	need := make(map[key]struct{})
	for _, a := range parsed {
		if a.Category == model.AtomCategoryProfile {
			need[key{model.AtomCategoryProfile, ""}] = struct{}{}
			continue
		}
		if a.Slug == "" {
			continue
		}
		need[key{a.Category, a.Slug}] = struct{}{}
	}
	for k := range need {
		var (
			rows *sql.Rows
			err  error
		)
		if k.category == model.AtomCategoryProfile {
			rows, err = w.db.QueryContext(ctx, `SELECT content FROM atoms WHERE category = 'profile'`)
		} else {
			rows, err = w.db.QueryContext(ctx, `SELECT content FROM atoms WHERE category = ? AND slug = ?`, k.category, k.slug)
		}
		if err != nil {
			w.log.Warn("l1 dedup lookup failed; inserting without suppression", "err", err)
			return parsed, 0
		}
		for rows.Next() {
			var content string
			if err := rows.Scan(&content); err != nil {
				rows.Close()
				return parsed, 0
			}
			existing[k] = append(existing[k], content)
		}
		rows.Close()
	}

	out := make([]llmAtom, 0, len(parsed))
	suppressed := 0
	for _, a := range parsed {
		var k key
		if a.Category == model.AtomCategoryProfile {
			k = key{model.AtomCategoryProfile, ""}
		} else {
			if a.Slug == "" {
				out = append(out, a)
				continue
			}
			k = key{a.Category, a.Slug}
		}
		dup := false
		for _, prev := range existing[k] {
			if textsim.Jaccard(a.Content, prev) >= atomDedupJaccard {
				dup = true
				break
			}
		}
		if dup {
			suppressed++
			continue
		}
		existing[k] = append(existing[k], a.Content) // in-batch dedup
		out = append(out, a)
	}
	return out, suppressed
}

// candidateSlugs retrieves existing active memory subjects (per category)
// relevant to this batch via FTS over memories (P2.1 retrieve-then-
// canonicalize, issue #27 task 2). Failure returns nil: extraction proceeds
// without candidates.
func (w *Worker) candidateSlugs(ctx context.Context, messagesJSONL string) []llm.SlugCandidate {
	query := candidateQuery(messagesJSONL)
	if query == "" {
		return nil
	}
	matches, err := recall.FTSSlugCandidates(ctx, w.db, query, 30)
	if err != nil || len(matches) == 0 {
		return nil
	}
	perCat := make(map[string][]string)
	for _, m := range matches {
		category, slug := memoryURISlug(m.URI)
		if category == "" || slug == "" {
			continue
		}
		slugs := perCat[category]
		seen := false
		for _, s := range slugs {
			if s == slug {
				seen = true
				break
			}
		}
		if !seen && len(slugs) < slugCandidatesPerCategory {
			perCat[category] = append(slugs, slug)
		}
	}
	var out []llm.SlugCandidate
	for category, slugs := range perCat {
		for _, slug := range slugs {
			out = append(out, llm.SlugCandidate{Category: category, Slug: slug})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}

// memoryURISlug splits rmb://<category>/<slug> into its parts; empty strings
// when the URI does not match the memory scheme.
func memoryURISlug(memoryURI string) (category, slug string) {
	rest := strings.TrimPrefix(memoryURI, "rmb://")
	if rest == memoryURI {
		return "", ""
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

// candidateQuery builds an OR-token FTS query from the batch's distinctive
// words (mirrors the L3 related-events query builder: drop stopwords, years,
// and sub-3-letter fragments so subject tokens dominate).
func candidateQuery(messagesJSONL string) string {
	tokens := textsim.Tokens(messagesJSONL)
	var words []string
	for t := range tokens {
		if len(t) < 3 || isYearToken(t) {
			continue
		}
		words = append(words, t)
	}
	if len(words) == 0 {
		return ""
	}
	sort.Strings(words)
	if len(words) > 12 {
		words = words[:12]
	}
	return strings.Join(words, " ")
}

func isYearToken(t string) bool {
	if len(t) != 4 {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (w *Worker) handleProcessError(ctx context.Context, sessionID string, cause error) error {
	return shared.MarkProcessError(ctx, w.db, w.log, pipeline.StageL1, sessionID, cause, w.now())
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
