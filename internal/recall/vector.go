package recall

import (
	"context"
	"database/sql"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"

	"github.com/colinleefish/rmb-desktop/internal/db"
)

func init() {
	sqlite_vec.Auto()
}

// VectorMemories returns memories ranked by cosine similarity to the query
// vector. Distance is computed inside SQLite via sqlite-vec's
// vec_distance_cosine, so vectors are no longer pulled into Go memory and
// ranked by a hand-rolled linear scan.
func VectorMemories(ctx context.Context, db *sql.DB, queryVec []float32, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}
	windowClause, windowArgs := tw.Clause("updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT uri,
		       COALESCE(substr(COALESCE(NULLIF(TRIM(abstract), ''), body), 1, 160), ''),
		       vec_distance_cosine(embedding, ?) AS distance,
		       version,
		       source_scene_uris
		FROM memories
		WHERE superseded_at IS NULL AND embedding IS NOT NULL AND archived_at IS NULL`+windowClause+`
		ORDER BY distance ASC
		LIMIT ?`, prependAny(blob, append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("vector memories: %w", err)
	}
	defer rows.Close()
	return scanVecMemoryMatches(rows, "memories")
}

// scanVecMemoryMatches scans the memory-tier vector result (uri, snippet,
// distance, version, source_scene_uris) and fills the extra Match fields.
func scanVecMemoryMatches(rows *sql.Rows, tier string) ([]Match, error) {
	var out []Match
	for rows.Next() {
		var uri, snippet, srcScenes string
		var distance float64
		var version int
		if err := rows.Scan(&uri, &snippet, &distance, &version, &srcScenes); err != nil {
			return nil, err
		}
		m := Match{
			URI: uri, Tier: tier, Rank: 1 - distance, Snippet: snippet, Version: version,
		}
		if scenes, err := db.UnmarshalStringArray(srcScenes); err == nil {
			m.SourceScenes = scenes
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func VectorScenes(ctx context.Context, db *sql.DB, queryVec []float32, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}
	windowClause, windowArgs := tw.Clause("updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT 'rmb://scenes/' || lower(id),
		       COALESCE(substr(COALESCE(NULLIF(TRIM(abstract), ''), body), 1, 160), ''),
		       vec_distance_cosine(embedding, ?) AS distance
		FROM scenes
		WHERE embedding IS NOT NULL`+windowClause+`
		ORDER BY distance ASC
		LIMIT ?`, prependAny(blob, append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("vector scenes: %w", err)
	}
	defer rows.Close()
	return scanVecMatches(rows, "scenes")
}

func VectorSkills(ctx context.Context, db *sql.DB, queryVec []float32, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}
	windowClause, windowArgs := tw.Clause("updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT uri,
		       COALESCE(substr(description, 1, 160), ''),
		       vec_distance_cosine(embedding, ?) AS distance
		FROM skills
		WHERE superseded_at IS NULL AND embedding IS NOT NULL`+windowClause+`
		ORDER BY distance ASC
		LIMIT ?`, prependAny(blob, append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("vector skills: %w", err)
	}
	defer rows.Close()
	return scanVecMatches(rows, "skills")
}

// VectorAtoms returns atom results ranked by cosine similarity to the query
// vector. Atoms carry the raw per-fact detail (audit Q8 style "resolver IPs"
// dead-ends), so a vector hit surfaces the evidence the one-line memories
// summarize over (plan §5 P1.1).
func VectorAtoms(ctx context.Context, db *sql.DB, queryVec []float32, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	blob, err := sqlite_vec.SerializeFloat32(queryVec)
	if err != nil {
		return nil, fmt.Errorf("serialize query vector: %w", err)
	}
	windowClause, windowArgs := tw.Clause("a.updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT 'rmb://atoms/' || lower(a.id) AS uri,
		       COALESCE(substr(a.content, 1, 160), '') AS snippet,
		       COALESCE((SELECT 'rmb://scenes/' || lower(s.id)
		                 FROM scenes s
		                 WHERE s.source_atoms LIKE '%"' || a.id || '"%'
		                 LIMIT 1), '') AS scene_uri,
		       'rmb://sessions/' || lower(a.session_id) AS session_uri,
		       vec_distance_cosine(a.embedding, ?) AS distance
		FROM atoms a
		WHERE a.embedding IS NOT NULL`+windowClause+`
		ORDER BY distance ASC
		LIMIT ?`, prependAny(blob, append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("vector atoms: %w", err)
	}
	defer rows.Close()
	return scanVecAtomMatches(rows, "atoms")
}

// scanVecAtomMatches reads an atom-tier vector row (uri, snippet,
// scene_uri, session_uri, distance) and appends drill-down annotations.
func scanVecAtomMatches(rows *sql.Rows, tier string) ([]Match, error) {
	var out []Match
	for rows.Next() {
		var uri, snippet, sceneURI, sessionURI string
		var distance float64
		if err := rows.Scan(&uri, &snippet, &sceneURI, &sessionURI, &distance); err != nil {
			return nil, err
		}
		out = append(out, annotateAtom(Match{URI: uri, Tier: tier, Rank: 1 - distance, Snippet: snippet}, sceneURI, sessionURI))
	}
	return out, rows.Err()
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
