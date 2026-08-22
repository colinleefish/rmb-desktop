package recall

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Usage-heat ranking (plan §10, issue #25 / D3).
//
// The memory tier's final score is:
//
//	final = rrf + α·log(1+heat) + β·e^(−age/14d)
//
//	heat = recall_stats.heat (decay-encoded frequency+recency of qualifying
//	       use — cat/meta only, never bare search impressions; issue #24)
//	age  = (now − memories.updated_at) in days
//
// Both bonus terms are bounded additive (≤~10% each of the median top-1 RRF
// score ≈ 1/61 ≈ 0.0164), so the boost only breaks near-ties:
//
// α·log(1+heat): with α=0.00035 the popularity term reaches ≈0.0016 (≈10%) at
//
//	heat≈100 and grows only logarithmically beyond — a hot-irrelevant memory
//	can never outrank a clearly-cold-relevant one.
//
// β·e^(−age/14d): at β=0.0016 a brand-new memory (heat=0, age≈0) gets a small
//
//	≈0.0016 cold-start novelty nudge; it decays to ~37% after 14d and ~12%
//	after 30d, then the memory must earn its position via relevance + heat.
const (
	heatAlphaDefault    = 0.00035 // α — popularity term, log(1+heat)
	heatBetaDefault     = 0.0016  // β — cold-start novelty term, e^(−age/14d)
	heatAgeHalfLifeDays = 14      // novelty decay half-life for the β term
)

// heatRankingEnabled is the package-level rollout switch (issue #25).
//
// DEFAULT OFF: while false the boost path is inert and Search output is
// byte-identical to the un-boosted path. It is flipped to true only after
// ≥30 days of accumulated heat telemetry (#24) plus a calibration pass — an
// OPERATIONAL decision post-merge, not faked in tests. It is read from
// RMB_HEAT_RANKING=1 so a single process-wide flag controls the rollout and
// a calibration run can force it on without a code change.
var (
	heatRankingEnabled = envBool("RMB_HEAT_RANKING", false)
	// α/β are env-tunable (issue #25 task 3) so calibration can adjust them
	// without a recompile; the constants above are the conservative defaults.
	heatAlpha = envFloat("RMB_HEAT_ALPHA", heatAlphaDefault)
	heatBeta  = envFloat("RMB_HEAT_BETA", heatBetaDefault)
)

// searchConfig carries per-call boost routing decided by SearchOptions.
type searchConfig struct {
	forceBoost *bool // nil = fall back to the package-level rollout flag
}

// SearchOption adjusts one recall search call.
type SearchOption func(*searchConfig)

// WithNoBoost forces the un-boosted (pure relevance) path for this call
// regardless of the heat-ranking rollout flag — the escape hatch so an agent
// or user can always get strict relevance ordering.
func WithNoBoost() SearchOption {
	return func(c *searchConfig) {
		off := false
		c.forceBoost = &off
	}
}

// WithHeatRanking explicitly enables or disables the boost for this call,
// overriding the package-level rollout flag (used by tests and calibration).
func WithHeatRanking(enabled bool) SearchOption {
	return func(c *searchConfig) {
		c.forceBoost = &enabled
	}
}

// boosting reports whether the heat boost applies to this search.
func (c searchConfig) boosting() bool {
	if c.forceBoost != nil {
		return *c.forceBoost
	}
	return heatRankingEnabled
}

// heatRankBoost returns the additive boost for a memory candidate: the
// popularity term from stored heat plus the cold-start novelty term from the
// memory's age. nowMS and updatedAtMS are unix-millisecond epochs.
func heatRankBoost(heat float64, updatedAtMS, nowMS int64) float64 {
	ageDays := float64(nowMS-updatedAtMS) / float64(24*time.Hour.Milliseconds())
	if ageDays < 0 {
		ageDays = 0
	}
	return heatAlpha*math.Log(1+heat) + heatBeta*math.Exp(-ageDays/heatAgeHalfLifeDays)
}

// memoryHeat is the boost inputs for a single memory candidate.
type memoryHeat struct {
	heat        float64 // recall_stats.heat (0 when no row)
	updatedAtMS int64   // memories.updated_at
}

// fetchMemoryHeat loads heat + updated_at for the candidate memory URIs in
// one batched query (left-joined against recall_stats so a missing stats row
// yields heat 0, which still lets the novelty term act on the age). Chunked
// to stay under SQLite's variable limit; the cost is bounded by the merged
// candidate list, never by k beyond it.
func fetchMemoryHeat(ctx context.Context, database *sql.DB, uris []string) (map[string]memoryHeat, error) {
	out := make(map[string]memoryHeat, len(uris))
	for start := 0; start < len(uris); start += embedFetchChunk {
		end := start + embedFetchChunk
		if end > len(uris) {
			end = len(uris)
		}
		chunk := uris[start:end]
		placeholders := make([]string, len(chunk))
		args := make([]any, len(chunk))
		for i, u := range chunk {
			placeholders[i] = "?"
			args[i] = u
		}
		rows, err := database.QueryContext(ctx,
			`SELECT m.uri, COALESCE(r.heat, 0), m.updated_at
			 FROM memories m
			 LEFT JOIN recall_stats r ON r.uri = m.uri
			 WHERE m.uri IN (`+strings.Join(placeholders, ",")+`)`,
			args...)
		if err != nil {
			return nil, fmt.Errorf("fetch memory heat: %w", err)
		}
		for rows.Next() {
			var uri string
			var h memoryHeat
			if err := rows.Scan(&uri, &h.heat, &h.updatedAtMS); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[uri] = h
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envFloat(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
