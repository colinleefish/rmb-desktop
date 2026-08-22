package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/correction"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/recall"
	"github.com/colinleefish/rmb-desktop/internal/textsim"
	"github.com/colinleefish/rmb-desktop/internal/worker/shared"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
	"github.com/google/uuid"
)

const distillDelay = 1 * time.Second

// l3Concurrency bounds how many buckets distill in parallel during one rollup.
const l3Concurrency = 8

// relatedEventsTopK caps how many related active events are injected into the
// L3 event distill prompt (retrieve-then-link, P2.2 / issue #28).
const relatedEventsTopK = 5

// graduationMinSessions is the distinct-session corroboration bar before a
// NEW preferences/entities subject graduates from append-only atoms/events
// into a rewritten memory (plan §5 P2.2, §9.3d). Below the bar the subject
// keeps accreting as atoms (immutable, still searchable via --scope=atom);
// this blocks single-utterance promotions like "call-user-daddy". Events are
// exempt (immutable, append-only by design); profile is exempt (singleton
// identity, correction-gated). K≈2–3 per plan; 2 keeps the bar at one
// independent corroboration.
const graduationMinSessions = 2

// Embedder is the optional embedding dependency for the pre-insert
// incumbent check (issue #27 task 3): it is satisfied by the embed worker's
// client. When nil, incumbent detection falls back to deterministic
// slug-normalization only (the cos path is skipped).
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// bodyUnchangedJaccard is the token-Jaccard bar at which a freshly distilled
// body counts as a paraphrase of the active body and does NOT bump the
// version (provenance columns are still refreshed). Second line of defense
// behind the materiality gate: reworded-but-same-information bodies (swap a
// couple of tokens in a >=12-token body => ~0.82) stay above it, while a
// genuine update (one new 5-token bullet on a 20-token body => ~0.7) falls
// below. Calibration fixtures live in consolidation_test.go.
const bodyUnchangedJaccard = 0.80

// bodyMinTokens guards the semantic body gate: token-set similarity is too
// twitchy on short bodies, so bodies with fewer distinct tokens only skip on
// exact equality.
const bodyMinTokens = 12

// incumbentCosThreshold is the embedding-cosine bar for merging a NEW
// subject into an existing same-category incumbent at persist time (issue
// #27 task 3). Set just under the near-identical band used by cross-tier
// suppression (#26: same-store random negatives max out ~0.95): same-subject
// paraphrase bodies measure above it, distinct subjects below.
const incumbentCosThreshold = 0.96

type MemoryDistiller interface {
	DistillMemory(ctx context.Context, category, slug, atomsJSON string, corrections []string, related []llm.RelatedEvent) (string, error)
}

type Worker struct {
	db    *sql.DB
	llm   MemoryDistiller
	embed Embedder
	cfg   config.PipelineConfig
	log   *slog.Logger
	now   func() time.Time
	reg   *debug.Registry
}

// NewWorker constructs the L3 memory worker. embed is optional (nil disables
// the cosine incumbent check; slug canonicalization still applies).
func NewWorker(database *sql.DB, llm MemoryDistiller, embed Embedder, cfg config.PipelineConfig, log *slog.Logger, reg *debug.Registry) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{db: database, llm: llm, embed: embed, cfg: cfg, log: log, now: time.Now, reg: reg}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.llm == nil {
		return fmt.Errorf("l3 worker requires llm client")
	}
	interval := w.cfg.L3PollInterval
	if interval <= 0 {
		return fmt.Errorf("invalid l3 poll interval")
	}
	shared.RunPoll(ctx, shared.PollOptions{
		Name:     "l3",
		Label:    "l3 memory",
		Interval: interval,
		Registry: w.reg,
		Log:      w.log,
		Cycle:    w.runOneCycle,
	})
	return nil
}

func (w *Worker) runOneCycle(ctx context.Context) {
	endCycle := w.reg.BeginCycle("l3")
	var cycleErr error
	defer func() { endCycle(cycleErr) }()

	workerlock.GlobalLock.Lock()
	defer workerlock.GlobalLock.Unlock()
	if err := w.rollup(ctx); err != nil {
		w.log.Error("l3 rollup failed", "err", err)
		cycleErr = err
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
	deferredBuckets := 0
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, l3Concurrency)
	)
	for _, bucket := range buckets {
		if ctx.Err() != nil {
			break
		}
		bucket := bucket
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			srcScenes := sourceSceneURIsFor(bucket, index)
			corrStatements, corrURIs := correction.SplitSummaries(corrByTarget[bucket.URI])

			// Graduation bar first (P2.2, §9.3d): a NEW preferences/entities
			// subject below the distinct-session corroboration bar keeps
			// accreting as atoms instead of being promoted to a memory.
			deferred, err := w.graduationDeferred(ctx, bucket)
			if err != nil {
				w.log.Warn("l3 graduation check failed", "uri", bucket.URI, "err", err)
				mu.Lock()
				transientPending = true
				mu.Unlock()
				return
			}
			if deferred {
				mu.Lock()
				deferredBuckets++
				mu.Unlock()
				return
			}

			unchanged, err := w.bucketUnchanged(ctx, bucket, srcScenes, corrURIs)
			if err != nil {
				w.log.Warn("l3 provenance check failed", "uri", bucket.URI, "err", err)
				mu.Lock()
				transientPending = true
				mu.Unlock()
				return
			}
			if unchanged {
				return
			}

			pm, err := w.distillBucket(ctx, bucket, corrStatements)
			if err != nil {
				if llm.IsTransientError(err) {
					w.log.Warn("l3 transient error", "uri", bucket.URI, "err", err)
					mu.Lock()
					transientPending = true
					mu.Unlock()
				} else {
					w.log.Warn("l3 bucket failed", "uri", bucket.URI, "err", err)
				}
				return
			}

			if err := w.persistMemory(ctx, bucket, pm, srcScenes, corrURIs); err != nil {
				if llm.IsTransientError(err) {
					mu.Lock()
					transientPending = true
					mu.Unlock()
				} else {
					w.log.Warn("l3 persist failed", "uri", bucket.URI, "err", err)
				}
				return
			}
		}()
	}
	wg.Wait()

	if deferredBuckets > 0 {
		w.log.Info("l3 graduation deferred (below corroboration bar; atoms kept)",
			"buckets", deferredBuckets, "min_sessions", graduationMinSessions)
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
	// Single-atom buckets don't need LLM distillation — there's nothing to
	// merge. Emit the fact directly so LLM failures (empty/truncated
	// responses) can't stall the whole rollup. Events are exempt: even a
	// single-atom event goes through the LLM so the structured body
	// (Decision/Rationale/Outcome/Related/Refs) and related-event linking
	// apply (P2.2, issue #28).
	if len(bucket.Atoms) == 1 && bucket.Category != model.AtomCategoryEvents {
		content := strings.TrimSpace(bucket.Atoms[0].Content)
		if content == "" {
			return ParsedMemory{}, fmt.Errorf("single-atom bucket has empty content")
		}
		abstract := content
		if r := []rune(content); len(r) > 200 {
			abstract = string(r[:200])
		}
		return ParsedMemory{Abstract: abstract, Body: content}, nil
	}

	// Retrieve-then-link (P2.2, issue #28): for event buckets, fetch the top
	// related active events and inject them into the prompt so a resolution
	// can reference the earlier problem event. Failure is non-fatal — distill
	// without links rather than stalling the rollup.
	var related []llm.RelatedEvent
	if bucket.Category == model.AtomCategoryEvents {
		r, err := w.relatedEvents(ctx, bucket)
		if err != nil {
			w.log.Warn("l3 related-event retrieval failed; distilling without links", "uri", bucket.URI, "err", err)
		} else {
			related = r
		}
	}

	chunks := chunkAtoms(bucket.Atoms, w.cfg.L3MaxAtoms)
	if len(chunks) == 1 {
		atomsJSON, err := serializeAtomsForLLM(chunks[0])
		if err != nil {
			return ParsedMemory{}, err
		}
		raw, err := w.llm.DistillMemory(ctx, bucket.Category, bucket.Slug, atomsJSON, corrections, related)
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
		raw, err := w.llm.DistillMemory(ctx, bucket.Category, bucket.Slug, atomsJSON, nil, related)
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

	// Reduce-path fix (issue #27 task 5, plan §9.3c): the final merge call
	// must see atom-level evidence, never distilled-only input — feeding
	// partials alone re-distills distilled text (rewrite-of-rewrite, the
	// erosion mechanism from arXiv:2605.12978). Partials ride along as
	// context; the facts are the atoms themselves (excerpted per atom to
	// bound the reduce prompt).
	reduceJSON, err := serializeReduceForLLM(bucket.Atoms, partials)
	if err != nil {
		return ParsedMemory{}, err
	}
	raw, err := w.llm.DistillMemory(ctx, bucket.Category, bucket.Slug, reduceJSON, corrections, related)
	if err != nil {
		return ParsedMemory{}, err
	}
	return parseDistillResponse(raw)
}

// graduationDeferred reports whether bucket is a NEW preferences/entities
// subject whose atoms do not yet span graduationMinSessions distinct
// sessions. Deferred subjects keep accreting as append-only atoms/events
// instead of being promoted to a rewritten memory (issue #28, plan §9.3d).
// Already-graduated subjects (an active memory exists) rewrite as before.
func (w *Worker) graduationDeferred(ctx context.Context, bucket Bucket) (bool, error) {
	if bucket.Category != model.AtomCategoryPreferences && bucket.Category != model.AtomCategoryEntities {
		return false, nil
	}
	var active int
	err := w.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memories WHERE uri = ? AND superseded_at IS NULL`, bucket.URI,
	).Scan(&active)
	if err != nil {
		return false, err
	}
	if active > 0 {
		return false, nil
	}
	return distinctSessionCount(bucket.Atoms) < graduationMinSessions, nil
}

// distinctSessionCount counts distinct source sessions across atoms.
func distinctSessionCount(atoms []model.Atom) int {
	seen := make(map[string]struct{}, len(atoms))
	for _, a := range atoms {
		if a.SessionID != "" {
			seen[a.SessionID] = struct{}{}
		}
	}
	return len(seen)
}

// bucketUnchanged implements the materiality gate (issue #27 task 1/6,
// plan §9.3a): a rewritten-category bucket is only re-distilled when its
// evidence actually changed. The primary signal is the atom-ID fingerprint
// stored on the active row (atoms are append-only, so an equal hash means
// zero new evidence — replacing the old source-scene gate that re-distilled
// the profile ~8x/day because every new session contributed a paraphrase
// atom). Rows written before migration 00012 carry no hash and fall back to
// scene-set equality.
func (w *Worker) bucketUnchanged(ctx context.Context, bucket Bucket, srcScenes, corrURIs []string) (bool, error) {
	var sourceScenesJSON, sourceCorrJSON, storedHash string
	var category string
	err := w.db.QueryRowContext(ctx, `
		SELECT source_scene_uris, source_correction_uris, category, COALESCE(source_atom_hash, '')
		FROM memories WHERE uri = ? AND superseded_at IS NULL`, bucket.URI,
	).Scan(&sourceScenesJSON, &sourceCorrJSON, &category, &storedHash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if bucket.Category == model.AtomCategoryEvents {
		return true, nil
	}
	existingCorr, err := db.UnmarshalStringArray(sourceCorrJSON)
	if err != nil {
		return false, err
	}
	corrUnchanged := equalStringSets(existingCorr, corrURIs)
	if storedHash != "" {
		// Materiality (P2.1): identical evidence fingerprint => skip unless
		// corrections changed. Any new atom => re-distill (N=1 with the L1
		// paraphrase suppression upstream, so only genuinely new facts fire).
		return textsim.HashIDs(atomIDs(bucket.Atoms)) == storedHash && corrUnchanged, nil
	}
	existing, err := db.UnmarshalStringArray(sourceScenesJSON)
	if err != nil {
		return false, err
	}
	return equalStringSets(existing, srcScenes) && corrUnchanged, nil
}

// bodyUnchanged reports whether a freshly distilled body carries no material
// delta over the active body: exact equality, or token-set Jaccard at/above
// bodyUnchangedJaccard on a body long enough for the signal to be reliable.
func bodyUnchanged(activeBody, newBody string) bool {
	if activeBody == newBody {
		return true
	}
	ta, tb := textsim.Tokens(activeBody), textsim.Tokens(newBody)
	if len(ta) < bodyMinTokens || len(tb) < bodyMinTokens {
		return false
	}
	return textsim.Jaccard(activeBody, newBody) >= bodyUnchangedJaccard
}

func atomIDs(atoms []model.Atom) []string {
	out := make([]string, 0, len(atoms))
	for _, a := range atoms {
		out = append(out, a.ID)
	}
	return out
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
	atomHash := textsim.HashIDs(atomIDs(bucket.Atoms))

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
		if err := insertMemory(ctx, tx, bucket, pm, sceneJSON, corrJSON, atomHash, 1, nowMS); err != nil {
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
		// Pre-insert incumbent check (issue #27 task 3): before minting a new
		// subject slug, look for an existing same-category active that IS the
		// same subject (deterministic slug normalization, optionally backed
		// by embedding cosine). Merge into the incumbent instead — the
		// redis-per-env replay must bump the incumbent's version, not create
		// a second slug. Hysteresis: the incumbent wins.
		inc, err := w.findIncumbent(ctx, bucket, pm)
		if err != nil {
			w.log.Warn("l3 incumbent lookup failed; inserting as new subject", "uri", bucket.URI, "err", err)
		} else if inc.uri != "" {
			if err := w.mergeIntoIncumbent(ctx, tx, bucket, pm, inc, sceneJSON, corrJSON, atomHash, nowMS); err != nil {
				return err
			}
			w.log.Info("l3 merged new subject into incumbent", "uri", bucket.URI, "incumbent", inc.uri)
			return tx.Commit()
		}
		if err := insertMemory(ctx, tx, bucket, pm, sceneJSON, corrJSON, atomHash, 1, nowMS); err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("load active memory: %w", err)
	}
	if activeBody == pm.Body {
		// Exact repeat: refresh provenance fingerprint only (the materiality
		// gate normally skips this distill; corrections can still land here).
		if err := refreshProvenance(ctx, tx, activeID, sceneJSON, corrJSON, atomHash, nowMS); err != nil {
			return err
		}
		return tx.Commit()
	}
	// Body-diff gate, generalized from exact-match to semantic (issue #27
	// task 6): a paraphrased body with no material delta does not bump the
	// version (profile rewritten ~8x/day on unchanged identity before).
	if bodyUnchanged(activeBody, pm.Body) {
		if err := refreshProvenance(ctx, tx, activeID, sceneJSON, corrJSON, atomHash, nowMS); err != nil {
			return err
		}
		return tx.Commit()
	}

	_, err = tx.ExecContext(ctx, `UPDATE memories SET superseded_at = ? WHERE id = ?`, nowMS, activeID)
	if err != nil {
		return fmt.Errorf("supersede memory: %w", err)
	}
	if err := insertMemory(ctx, tx, bucket, pm, sceneJSON, corrJSON, atomHash, version+1, nowMS); err != nil {
		return err
	}
	return tx.Commit()
}

type incumbent struct {
	uri     string
	id      string
	version int
}

// findIncumbent locates an existing same-category active memory that denotes
// the same subject as a NEW bucket (no active row under bucket.URI):
//   - slug path (deterministic, always on): a slug that normalizes equal to
//     the bucket slug ("doc-language" vs "docs-language");
//   - cos path (optional, embedder wired): active whose embedding is near
//     the candidate body's (cos >= incumbentCosThreshold).
//
// It never matches the bucket's own URI.
func (w *Worker) findIncumbent(ctx context.Context, bucket Bucket, pm ParsedMemory) (incumbent, error) {
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, uri, COALESCE(slug, ''), version FROM memories
		WHERE category = ? AND superseded_at IS NULL AND archived_at IS NULL AND uri <> ?`,
		bucket.Category, bucket.URI)
	if err != nil {
		return incumbent{}, err
	}
	defer rows.Close()
	var (
		bySlug  []incumbent
		anyList []incumbent
	)
	for rows.Next() {
		var inc incumbent
		var slug string
		if err := rows.Scan(&inc.id, &inc.uri, &slug, &inc.version); err != nil {
			return incumbent{}, err
		}
		if slug != "" && textsim.SlugEqual(slug, bucket.Slug) {
			bySlug = append(bySlug, inc)
		}
		anyList = append(anyList, inc)
	}
	if err := rows.Err(); err != nil {
		return incumbent{}, err
	}
	if len(bySlug) > 0 {
		// Deterministic: lowest URI wins ties.
		sort.Slice(bySlug, func(i, j int) bool { return bySlug[i].uri < bySlug[j].uri })
		return bySlug[0], nil
	}

	if w.embed == nil || len(anyList) == 0 {
		return incumbent{}, nil
	}
	vecs, err := w.embed.Embed(ctx, []string{pm.Abstract})
	if err != nil || len(vecs) != 1 || len(vecs[0]) == 0 {
		return incumbent{}, err
	}
	queryVec := vecs[0]
	best := incumbent{}
	bestCos := 0.0
	for _, inc := range anyList {
		blob, dim, err := storedEmbedding(ctx, w.db, inc.id)
		if err != nil || blob == nil || dim != len(queryVec) {
			continue
		}
		if cos := recall.CosineSim(queryVec, blob); cos > bestCos {
			bestCos, best = cos, inc
		}
	}
	if bestCos >= incumbentCosThreshold {
		return best, nil
	}
	return incumbent{}, nil
}

// mergeIntoIncumbent rewrites the incumbent under ITS OWN uri with the new
// evidence: supersede the incumbent's active row, insert version+1 under the
// incumbent uri. The variant bucket slug never mints a new memory uri.
func (w *Worker) mergeIntoIncumbent(ctx context.Context, tx *sql.Tx, bucket Bucket, pm ParsedMemory, inc incumbent, sceneJSON, corrJSON, atomHash string, nowMS int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE memories SET superseded_at = ? WHERE id = ?`, nowMS, inc.id)
	if err != nil {
		return fmt.Errorf("supersede incumbent: %w", err)
	}
	incBucket := Bucket{Category: bucket.Category, Slug: inc.uri, URI: inc.uri}
	if slug, ok := strings.CutPrefix(inc.uri, "rmb://"+bucket.Category+"/"); ok {
		incBucket.Slug = slug
	}
	return insertMemory(ctx, tx, incBucket, pm, sceneJSON, corrJSON, atomHash, inc.version+1, nowMS)
}

func storedEmbedding(ctx context.Context, database *sql.DB, id string) ([]float32, int, error) {
	var blob []byte
	err := database.QueryRowContext(ctx, `SELECT embedding FROM memories WHERE id = ?`, id).Scan(&blob)
	if err == sql.ErrNoRows || err != nil {
		return nil, 0, err
	}
	if len(blob) == 0 {
		return nil, 0, nil
	}
	vec := recall.DecodeVecFloat32(blob)
	if len(vec) == 0 {
		return nil, 0, nil
	}
	return vec, len(vec), nil
}

func refreshProvenance(ctx context.Context, tx *sql.Tx, id, sceneJSON, corrJSON, atomHash string, nowMS int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE memories SET source_scene_uris = ?, source_correction_uris = ?, source_atom_hash = ?, updated_at = ?
		WHERE id = ?`, sceneJSON, corrJSON, atomHash, nowMS, id)
	return err
}

func insertMemory(ctx context.Context, tx *sql.Tx, bucket Bucket, pm ParsedMemory, sceneJSON, corrJSON, atomHash string, version int, nowMS int64) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var slugPtr *string
	if bucket.Slug != "" {
		slugPtr = &bucket.Slug
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memories (id, uri, category, slug, version, abstract, body, source_scene_uris, source_correction_uris, source_atom_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), bucket.URI, bucket.Category, slugPtr, version, pm.Abstract, pm.Body, sceneJSON, corrJSON, atomHash, nowMS, nowMS,
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
	return db.ScanAtomRows(rows)
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
