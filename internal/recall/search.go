package recall

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

const (
	vectorWeight = 0.7
	ftsWeight    = 0.3
)

// QueryEmbedder embeds a query string for vector recall. Nil disables vector leg.
type QueryEmbedder func(ctx context.Context, query string) ([]float32, error)

// Service runs hybrid recall over SQLite.
type Service struct {
	DB *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{DB: db}
}

// Search runs hybrid recall (vector + FTS fused 70/30 per tier). Without an
// embedder, falls back to FTS-only (D21).
func (s *Service) Search(ctx context.Context, embed QueryEmbedder, query string, k int, scopes []string) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}

	wantMemory, wantScene := false, false
	for _, sc := range scopes {
		switch sc {
		case "memory":
			wantMemory = true
		case "scene":
			wantScene = true
		case "skill":
			// skills deferred post-M4 schema
		default:
			return nil, fmt.Errorf("invalid scope %q", sc)
		}
	}
	if !wantMemory && !wantScene {
		wantMemory, wantScene = true, true
	}

	perList := k * 2
	var queryVec []float32
	var hasVector bool
	if embed != nil {
		vec, err := embed(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("embed query: %w", err)
		}
		if len(vec) > 0 {
			queryVec = vec
			hasVector = true
		}
	}

	type tierHit struct {
		match Match
		score float64
	}
	var merged []tierHit

	fuseTier := func(vecHits, ftsHits []Match) {
		vecURIs := make([]string, len(vecHits))
		for i, m := range vecHits {
			vecURIs[i] = m.URI
		}
		ftsURIs := make([]string, len(ftsHits))
		for i, m := range ftsHits {
			ftsURIs[i] = m.URI
		}
		ranked := FuseRRF(vecURIs, ftsURIs, 0, vectorWeight, ftsWeight)
		meta := map[string]Match{}
		for _, m := range append(vecHits, ftsHits...) {
			if _, ok := meta[m.URI]; !ok {
				meta[m.URI] = m
			}
		}
		for _, r := range ranked {
			m := meta[r.URI]
			m.Rank = r.Score
			merged = append(merged, tierHit{match: m, score: r.Score})
		}
	}

	if wantMemory {
		fts, err := FTSMemories(ctx, s.DB, query, perList)
		if err != nil {
			return nil, err
		}
		if hasVector {
			vec, err := VectorMemories(ctx, s.DB, queryVec, perList)
			if err != nil {
				return nil, err
			}
			fuseTier(vec, fts)
		} else {
			for _, m := range fts {
				merged = append(merged, tierHit{match: m, score: m.Rank})
			}
		}
	}

	if wantScene {
		fts, err := FTSScenes(ctx, s.DB, query, perList)
		if err != nil {
			return nil, err
		}
		if hasVector {
			vec, err := VectorScenes(ctx, s.DB, queryVec, perList)
			if err != nil {
				return nil, err
			}
			fuseTier(vec, fts)
		} else {
			for _, m := range fts {
				merged = append(merged, tierHit{match: m, score: m.Rank})
			}
		}
	}

	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].score == merged[j].score {
			return merged[i].match.URI < merged[j].match.URI
		}
		return merged[i].score > merged[j].score
	})

	seen := make(map[string]struct{})
	out := make([]Match, 0, k)
	for _, h := range merged {
		if _, dup := seen[h.match.URI]; dup {
			continue
		}
		seen[h.match.URI] = struct{}{}
		out = append(out, h.match)
		if len(out) >= k {
			break
		}
	}
	return out, nil
}
