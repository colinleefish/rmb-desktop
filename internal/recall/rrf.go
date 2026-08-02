package recall

import "sort"

// Ranked is a search hit with a fused score.
type Ranked struct {
	URI   string
	Score float64
}

// FuseRRF merges ranked lists with reciprocal rank fusion.
// vectorWeight and ftsWeight should sum to 1.0 (e.g. 0.7 and 0.3).
func FuseRRF(vectorHits, ftsHits []string, k int, vectorWeight, ftsWeight float64) []Ranked {
	const rrfK = 60.0
	scores := map[string]float64{}

	add := func(hits []string, weight float64) {
		for i, uri := range hits {
			scores[uri] += weight * (1.0 / (rrfK + float64(i+1)))
		}
	}
	add(vectorHits, vectorWeight)
	add(ftsHits, ftsWeight)

	out := make([]Ranked, 0, len(scores))
	for uri, score := range scores {
		out = append(out, Ranked{URI: uri, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].URI < out[j].URI
		}
		return out[i].Score > out[j].Score
	})
	if k > 0 && len(out) > k {
		out = out[:k]
	}
	return out
}
