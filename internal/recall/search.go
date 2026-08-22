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
// embedder, falls back to FTS-only (D21). A non-zero tw filters every tier
// by its updated_at column.
func (s *Service) Search(ctx context.Context, embed QueryEmbedder, query string, k int, scopes []string, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}

	wantMemory, wantScene, wantSkill, wantAtom := false, false, false, false
	for _, sc := range scopes {
		switch sc {
		case "memory":
			wantMemory = true
		case "scene":
			wantScene = true
		case "skill":
			wantSkill = true
		case "atom":
			wantAtom = true
		default:
			return nil, fmt.Errorf("invalid scope %q", sc)
		}
	}
	if !wantMemory && !wantScene && !wantSkill && !wantAtom {
		wantMemory, wantScene, wantSkill = true, true, true
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
		fts, err := FTSMemories(ctx, s.DB, query, perList, tw)
		if err != nil {
			return nil, err
		}
		if hasVector {
			vec, err := VectorMemories(ctx, s.DB, queryVec, perList, tw)
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
		fts, err := FTSScenes(ctx, s.DB, query, perList, tw)
		if err != nil {
			return nil, err
		}
		if hasVector {
			vec, err := VectorScenes(ctx, s.DB, queryVec, perList, tw)
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

	if wantAtom {
		// Atoms are the raw per-fact evidence tier. They are an explicit
		// --scope=atom tier (never default) so detail questions can reach the
		// content that one-line memories only summarize (plan §5 P1.1, C6).
		fts, err := FTSAtoms(ctx, s.DB, query, perList, tw)
		if err != nil {
			return nil, err
		}
		if hasVector {
			vec, err := VectorAtoms(ctx, s.DB, queryVec, perList, tw)
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

	if wantSkill {
		// Skills are FTS-only: their descriptions are terse and domain-homogeneous,
		// so the vector leg drifts toward generic ops language and lets an
		// irrelevant-but-generic skill (e.g. draft-aliyun-procurement-ticket)
		// pollute every result list (plan §5 P0.2, C4). Lexical match on the
		// distinctive name/description is the correct signal.
		fts, err := FTSSkills(ctx, s.DB, query, perList, tw)
		if err != nil {
			return nil, err
		}
		for _, m := range fts {
			merged = append(merged, tierHit{match: m, score: m.Rank})
		}
	}

	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].score == merged[j].score {
			return merged[i].match.URI < merged[j].match.URI
		}
		return merged[i].score > merged[j].score
	})

	// Link-based scene suppression: a scene that is the source_scene_uris of a
	// higher-ranked memory is evidence, not a separate result. Drop it and
	// annotate the owning memory so agents can drill down deterministically
	// (plan §9.2: memories are the index, scenes are the evidence).
	sceneOwner := map[string]int{} // scene uri -> merged index of owning memory
	sceneIdx := map[string]int{}   // scene uri -> merged index of the scene hit itself
	for i := range merged {
		if merged[i].match.Tier == "scenes" {
			sceneIdx[merged[i].match.URI] = i
			continue
		}
		for _, sc := range merged[i].match.SourceScenes {
			if _, ok := sceneOwner[sc]; !ok {
				sceneOwner[sc] = i
			}
		}
	}
	keep := make([]tierHit, 0, len(merged))
	suppressed := map[string]string{} // scene uri -> owner memory uri (annotation)
	for i := range merged {
		m := merged[i]
		if m.match.Tier == "scenes" {
			if owner, ok := sceneOwner[m.match.URI]; ok && owner != i {
				suppressed[m.match.URI] = merged[owner].match.URI
				continue
			}
		}
		keep = append(keep, m)
	}
	// Annotate owners with suppressed evidence scenes (first one only, avoid
	// clobbering).
	annotated := map[string]bool{}
	for sc, ownerURI := range suppressed {
		for i := range keep {
			if keep[i].match.URI == ownerURI && !annotated[ownerURI] {
				keep[i].match.Snippet += fmt.Sprintf(" (+scene depth: %s)", sc)
				annotated[ownerURI] = true
				break
			}
		}
	}

	// Skill cap: outside an explicit skill-only scope, keep only the single
	// highest-ranked skill so the generic-match skill cannot fill the list.
	skillOnly := wantSkill && !wantMemory && !wantScene && !wantAtom
	skillSeen := 0
	filtered := keep[:0]
	for _, th := range keep {
		if th.match.Tier == "skills" && !skillOnly {
			if skillSeen >= 1 {
				continue
			}
			skillSeen++
		}
		filtered = append(filtered, th)
	}
	keep = filtered

	seen := make(map[string]struct{})
	out := make([]Match, 0, k)
	for _, h := range keep {
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
