// Package hygiene implements the standing health checks and superseded-row
// garbage collection (issue #33, plan §5 P3.4 / §9.3e).
//
// Everything here is deterministic SQL over the store: no LLM calls, no
// auto-mutations. Doctor REPORTS; the user decides. The only mutating entry
// point is SupersededGC with dryRun=false, which deletes superseded (already
// inactive) version rows only — never an active memory.
package hygiene

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/recall"
)

// Default GC policy: per URI keep at least the newest 3 superseded versions,
// or everything newer than 90 days — whichever retains MORE rows.
const (
	DefaultKeepVersions = 3
	DefaultKeepDays     = 90
)

// HighVersionThreshold flags memories whose rewrite chain grew past it (the
// audit found chains up to 131 versions — erosion candidates for
// `doctor reconsolidate`).
const HighVersionThreshold = 20

// MaxBodyChars mirrors the worker's distill-time body cap (4k); the report
// counts legacy bodies that exceed it.
const MaxBodyChars = 4096

// DatePrefixRE matches the event slug date convention enforced at extract
// time (P2.1); the report counts legacy events without it.
var DatePrefixRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)

// negationTokens: a deterministic contradiction heuristic — near-duplicate
// bodies where exactly one side carries a negation are candidates for
// review (labeled as such; no auto-correction).
var negationTokens = []string{" not ", " never ", " don't ", " avoid ", " no longer ", "不再", "不要", "不"}

// GCStats reports what a GC run (dry or real) found.
type GCStats struct {
	URIsScanned    int   `json:"uris_scanned"`
	SupersededRows int   `json:"superseded_rows"`
	KeepRows       int   `json:"keep_rows"`
	DeletedRows    int   `json:"deleted_rows"`
	DryRun         bool  `json:"dry_run"`
	ReclaimedBytes int64 `json:"reclaimed_bytes"`
}

// SupersededGC garbage-collects superseded memory rows per URI, keeping the
// newer of (keepVersions newest, keepDays freshest). Active rows
// (superseded_at IS NULL) are never touched — this compacts history, it
// does not forget. dryRun reports the would-be deletions without deleting.
func SupersededGC(ctx context.Context, database *sql.DB, keepVersions, keepDays int, dryRun bool) (GCStats, error) {
	if keepVersions <= 0 {
		keepVersions = DefaultKeepVersions
	}
	if keepDays <= 0 {
		keepDays = DefaultKeepDays
	}
	stats := GCStats{DryRun: dryRun}

	rows, err := database.QueryContext(ctx, `
		SELECT uri, id, updated_at, length(COALESCE(body,'')) + length(COALESCE(abstract,'')) AS bytes
		FROM memories WHERE superseded_at IS NOT NULL
		ORDER BY uri ASC, updated_at DESC`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	var deleteIDs []string
	var currentURI string
	var rank int
	var uriCount int
	for rows.Next() {
		var uri, id string
		var updatedAt, bytes int64
		if err := rows.Scan(&uri, &id, &updatedAt, &bytes); err != nil {
			return stats, err
		}
		if uri != currentURI {
			currentURI = uri
			rank = 0
			uriCount++
		}
		stats.SupersededRows++
		age := time.Since(time.UnixMilli(updatedAt))
		if rank < keepVersions || age < time.Duration(keepDays)*24*time.Hour {
			stats.KeepRows++
		} else {
			deleteIDs = append(deleteIDs, id)
			stats.ReclaimedBytes += bytes
		}
		rank++
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}
	stats.URIsScanned = uriCount
	stats.DeletedRows = len(deleteIDs)

	if dryRun || len(deleteIDs) == 0 {
		return stats, nil
	}
	for _, id := range deleteIDs {
		if _, err := database.ExecContext(ctx, `DELETE FROM memories WHERE id = ? AND superseded_at IS NOT NULL`, id); err != nil {
			return stats, fmt.Errorf("delete superseded row: %w", err)
		}
	}
	return stats, nil
}

// DupPair is a same-category active-memory pair whose stored embeddings are
// near-identical (cos > threshold) — a duplicate candidate for review.
type DupPair struct {
	A        string  `json:"a"`
	B        string  `json:"b"`
	Category string  `json:"category"`
	Cos      float64 `json:"cos"`
}

// ContradictionCandidate is a near-duplicate pair where exactly one side
// carries a negation token — flagged for human review, never auto-fixed.
type ContradictionCandidate struct {
	DupPair
	NegatedSide string `json:"negated_side"`
}

// HealthReport is the doctor's on-demand snapshot of store health. Every
// field is a report, not an action (issue #33 acceptance).
type HealthReport struct {
	GeneratedAt int64 `json:"generated_at"`

	// Version history.
	ActiveMemories    int `json:"active_memories"`
	SupersededRows    int `json:"superseded_rows"`
	GCReclaimableRows int `json:"gc_reclaimable_rows"`
	HighVersionCount  int `json:"high_version_count"`

	// Content hygiene.
	DatelessEventSlugs int `json:"dateless_event_slugs"`
	BodiesOverCap      int `json:"bodies_over_cap"`

	// Evidence graph.
	OrphanScenes int `json:"orphan_scenes"`

	// Retrieval health (deterministic views; the recall-stats doctor adds
	// zero-cat rate + heat concentration at the handler level).
	DuplicatePairs          int                      `json:"duplicate_pairs"`
	TopDuplicatePairs       []DupPair                `json:"top_duplicate_pairs,omitempty"`
	ContradictionCandidates []ContradictionCandidate `json:"contradiction_candidates,omitempty"`
}

// Report scans the store for the doctor's health snapshot. dupLimit caps how
// many duplicate pairs are computed/reported (the full pairwise scan is
// bounded per category chunk; see duplicatePairs).
func Report(ctx context.Context, database *sql.DB, dupLimit int) (HealthReport, error) {
	out := HealthReport{GeneratedAt: time.Now().UTC().UnixMilli(), TopDuplicatePairs: []DupPair{}, ContradictionCandidates: []ContradictionCandidate{}}
	if dupLimit <= 0 {
		dupLimit = 50
	}

	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE superseded_at IS NULL`).Scan(&out.ActiveMemories); err != nil {
		return out, err
	}
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE superseded_at IS NOT NULL`).Scan(&out.SupersededRows); err != nil {
		return out, err
	}
	if gc, err := SupersededGC(ctx, database, DefaultKeepVersions, DefaultKeepDays, true); err == nil {
		out.GCReclaimableRows = gc.DeletedRows
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memories WHERE superseded_at IS NULL AND version >= ?`, HighVersionThreshold).Scan(&out.HighVersionCount); err != nil {
		return out, err
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memories
		WHERE superseded_at IS NULL AND category = 'events'
		  AND (slug IS NULL OR slug NOT GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-*')`).Scan(&out.DatelessEventSlugs); err != nil {
		return out, err
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memories
		WHERE superseded_at IS NULL AND length(COALESCE(body,'')) > ?`, MaxBodyChars).Scan(&out.BodiesOverCap); err != nil {
		return out, err
	}
	if err := orphanSceneCount(ctx, database).Scan(&out.OrphanScenes); err != nil {
		return out, err
	}

	pairs, contra, err := duplicatePairs(ctx, database, dupLimit)
	if err != nil {
		return out, err
	}
	out.TopDuplicatePairs = pairs
	out.DuplicatePairs = len(pairs)
	out.ContradictionCandidates = contra
	return out, nil
}

func orphanSceneCount(ctx context.Context, database *sql.DB) *sql.Row {
	return database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM scenes s
		WHERE NOT EXISTS (
			SELECT 1 FROM memories m
			WHERE m.superseded_at IS NULL
			  AND m.source_scene_uris LIKE '%"' || substr(s.id, 1, 8) || '%'
			  AND m.source_scene_uris LIKE '%' || s.id || '%'
		)`)
}

// duplicatePairs finds same-category active memories with stored embeddings
// above a near-identical cosine (0.90 per issue #33 — report bar, looser
// than the 0.98 suppression bar). Chunks per category to bound memory;
// stops after limit pairs.
func duplicatePairs(ctx context.Context, database *sql.DB, limit int) ([]DupPair, []ContradictionCandidate, error) {
	const cosThreshold = 0.90
	categories := []string{"profile", "preferences", "entities", "events"}
	var pairs []DupPair
	for _, category := range categories {
		if len(pairs) >= limit {
			break
		}
		rows, err := database.QueryContext(ctx, `
			SELECT id, uri, COALESCE(embedding, x''), COALESCE(body,'')
			FROM memories
			WHERE category = ? AND superseded_at IS NULL AND embedding IS NOT NULL
			ORDER BY updated_at DESC
			LIMIT 4000`, category)
		if err != nil {
			return nil, nil, err
		}
		type row struct {
			uri  string
			vec  []float32
			body string
		}
		var list []row
		for rows.Next() {
			var id string
			var blob []byte
			var r row
			if err := rows.Scan(&id, &r.uri, &blob, &r.body); err != nil {
				rows.Close()
				return nil, nil, err
			}
			r.vec = recall.DecodeVecFloat32(blob)
			if len(r.vec) > 0 {
				list = append(list, r)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, nil, err
		}
		for i := 0; i < len(list) && len(pairs) < limit; i++ {
			for j := i + 1; j < len(list) && len(pairs) < limit; j++ {
				if len(list[i].vec) != len(list[j].vec) {
					continue
				}
				cos := recall.CosineSim(list[i].vec, list[j].vec)
				if cos >= cosThreshold {
					pairs = append(pairs, DupPair{A: list[i].uri, B: list[j].uri, Category: category, Cos: cos})
				}
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Cos > pairs[j].Cos })

	// Contradiction heuristic: near-dup pairs where exactly one body has a
	// negation token.
	var contra []ContradictionCandidate
	for _, p := range pairs {
		aBody, bBody := bodyFor(database, p.A), bodyFor(database, p.B)
		aNeg, bNeg := hasNegation(aBody), hasNegation(bBody)
		if aNeg != bNeg {
			c := ContradictionCandidate{DupPair: p}
			if aNeg {
				c.NegatedSide = p.A
			} else {
				c.NegatedSide = p.B
			}
			contra = append(contra, c)
		}
	}
	return pairs, contra, nil
}

func bodyFor(database *sql.DB, uri string) string {
	var body string
	_ = database.QueryRow(`SELECT COALESCE(body,'') FROM memories WHERE uri = ? AND superseded_at IS NULL`, uri).Scan(&body)
	return body
}

func hasNegation(body string) bool {
	lower := " " + strings.ToLower(body) + " "
	for _, tok := range negationTokens {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}
