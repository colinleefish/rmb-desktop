package recall

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	sqlite_vec.Auto()
}

type vectorRow struct {
	URI     string
	Snippet string
	Vec     []float32
}

func VectorMemories(ctx context.Context, db *sql.DB, queryVec []float32, k int) ([]Match, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT uri,
		       COALESCE(substr(COALESCE(NULLIF(TRIM(abstract), ''), body), 1, 160), ''),
		       embedding
		FROM memories
		WHERE superseded_at IS NULL AND embedding IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("vector memories: %w", err)
	}
	defer rows.Close()
	return rankVectors(rows, queryVec, k, "memories")
}

func VectorScenes(ctx context.Context, db *sql.DB, queryVec []float32, k int) ([]Match, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT 'rmb://scenes/' || lower(id),
		       COALESCE(substr(COALESCE(NULLIF(TRIM(abstract), ''), body), 1, 160), ''),
		       embedding
		FROM scenes
		WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("vector scenes: %w", err)
	}
	defer rows.Close()
	return rankVectors(rows, queryVec, k, "scenes")
}

func rankVectors(rows *sql.Rows, queryVec []float32, k int, tier string) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	var items []vectorRow
	for rows.Next() {
		var uri, snippet string
		var blob []byte
		if err := rows.Scan(&uri, &snippet, &blob); err != nil {
			return nil, err
		}
		vec, err := deserializeFloat32(blob)
		if err != nil {
			continue
		}
		items = append(items, vectorRow{URI: uri, Snippet: snippet, Vec: vec})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type scored struct {
		row   vectorRow
		score float64
	}
	scoredItems := make([]scored, 0, len(items))
	for _, item := range items {
		scoredItems = append(scoredItems, scored{item, cosineSimilarity(queryVec, item.Vec)})
	}
	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].score > scoredItems[j].score
	})
	if len(scoredItems) > k {
		scoredItems = scoredItems[:k]
	}

	out := make([]Match, 0, len(scoredItems))
	for _, s := range scoredItems {
		out = append(out, Match{
			URI:     s.row.URI,
			Tier:    tier,
			Rank:    s.score,
			Snippet: s.row.Snippet,
		})
	}
	return out, nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func deserializeFloat32(blob []byte) ([]float32, error) {
	if len(blob)%4 != 0 {
		return nil, fmt.Errorf("invalid float32 blob length %d", len(blob))
	}
	out := make([]float32, len(blob)/4)
	for i := range out {
		u := binary.LittleEndian.Uint32(blob[i*4 : i*4+4])
		out[i] = math.Float32frombits(u)
	}
	return out, nil
}
