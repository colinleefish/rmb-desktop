package recall

import (
	"context"
	"database/sql"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func init() {
	sqlite_vec.Auto()
}

// VectorMemories returns memories ranked by cosine similarity to the query
// vector. Distance is computed inside SQLite via sqlite-vec's
// vec_distance_cosine, so vectors are no longer pulled into Go memory and
// ranked by a hand-rolled linear scan.
func VectorMemories(ctx context.Context, db *sql.DB, queryVec []float32, k int) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT uri,
		       COALESCE(substr(COALESCE(NULLIF(TRIM(abstract), ''), body), 1, 160), ''),
		       vec_distance_cosine(embedding, ?) AS distance
		FROM memories
		WHERE superseded_at IS NULL AND embedding IS NOT NULL
		ORDER BY distance ASC
		LIMIT ?`, blob, k)
	if err != nil {
		return nil, fmt.Errorf("vector memories: %w", err)
	}
	defer rows.Close()
	return scanVecMatches(rows, "memories")
}

func VectorScenes(ctx context.Context, db *sql.DB, queryVec []float32, k int) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT 'rmb://scenes/' || lower(id),
		       COALESCE(substr(COALESCE(NULLIF(TRIM(abstract), ''), body), 1, 160), ''),
		       vec_distance_cosine(embedding, ?) AS distance
		FROM scenes
		WHERE embedding IS NOT NULL
		ORDER BY distance ASC
		LIMIT ?`, blob, k)
	if err != nil {
		return nil, fmt.Errorf("vector scenes: %w", err)
	}
	defer rows.Close()
	return scanVecMatches(rows, "scenes")
}

func VectorSkills(ctx context.Context, db *sql.DB, queryVec []float32, k int) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT uri,
		       COALESCE(substr(description, 1, 160), ''),
		       vec_distance_cosine(embedding, ?) AS distance
		FROM skills
		WHERE superseded_at IS NULL AND embedding IS NOT NULL
		ORDER BY distance ASC
		LIMIT ?`, blob, k)
	if err != nil {
		return nil, fmt.Errorf("vector skills: %w", err)
	}
	defer rows.Close()
	return scanVecMatches(rows, "skills")
}

// scanVecMatches converts vec_distance_cosine (0 = identical, up to 2 = opposite)
// back into a similarity score (1 = identical, 0 = unrelated) for Rank.
func scanVecMatches(rows *sql.Rows, tier string) ([]Match, error) {
	var out []Match
	for rows.Next() {
		var uri, snippet string
		var distance float64
		if err := rows.Scan(&uri, &snippet, &distance); err != nil {
			return nil, err
		}
		out = append(out, Match{
			URI:     uri,
			Tier:    tier,
			Rank:    1 - distance,
			Snippet: snippet,
		})
	}
	return out, rows.Err()
}
