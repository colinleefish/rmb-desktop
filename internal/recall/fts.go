package recall

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/db"
)

// EscapeFTSQuery prepares a user query for FTS5 MATCH (phrase per token).
func EscapeFTSQuery(query string) string {
	parts := strings.Fields(strings.TrimSpace(query))
	if len(parts) == 0 {
		return `""`
	}
	for i, p := range parts {
		p = strings.ReplaceAll(p, `"`, `""`)
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, " ")
}

func FTSMemories(ctx context.Context, db *sql.DB, query string, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	windowClause, windowArgs := tw.Clause("m.updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT m.uri,
		       COALESCE(substr(COALESCE(NULLIF(TRIM(m.abstract), ''), m.body), 1, 160), '') AS snippet,
		       m.version,
		       m.source_scene_uris
		FROM memories m
		INNER JOIN memories_fts fts ON fts.rowid = m.rowid
		WHERE memories_fts MATCH ? AND m.superseded_at IS NULL`+windowClause+`
		ORDER BY bm25(memories_fts)
		LIMIT ?`, prependArgs(EscapeFTSQuery(query), append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("fts memories: %w", err)
	}
	defer rows.Close()
	return scanFTSMemoryMatches(rows, "memories")
}

func FTSScenes(ctx context.Context, db *sql.DB, query string, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	windowClause, windowArgs := tw.Clause("s.updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT 'rmb://scenes/' || lower(s.id) AS uri,
		       COALESCE(substr(COALESCE(NULLIF(TRIM(s.abstract), ''), s.body), 1, 160), '') AS snippet
		FROM scenes s
		INNER JOIN scenes_fts fts ON fts.rowid = s.rowid
		WHERE scenes_fts MATCH ?`+windowClause+`
		ORDER BY bm25(scenes_fts)
		LIMIT ?`, prependArgs(EscapeFTSQuery(query), append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("fts scenes: %w", err)
	}
	defer rows.Close()
	return scanFTSMatches(rows, "scenes")
}

func FTSSkills(ctx context.Context, db *sql.DB, query string, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	windowClause, windowArgs := tw.Clause("s.updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT s.uri,
		       COALESCE(substr(s.description, 1, 160), '') AS snippet
		FROM skills s
		INNER JOIN skills_fts fts ON fts.rowid = s.rowid
		WHERE skills_fts MATCH ? AND s.superseded_at IS NULL`+windowClause+`
		ORDER BY bm25(skills_fts)
		LIMIT ?`, prependArgs(EscapeFTSQuery(query), append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("fts skills: %w", err)
	}
	defer rows.Close()
	return scanFTSMatches(rows, "skills")
}

// scanFTSMemoryMatches scans the memory-tier FTS result (uri, snippet,
// version, source_scene_uris) and fills the extra Match fields.
func scanFTSMemoryMatches(rows *sql.Rows, tier string) ([]Match, error) {
	var out []Match
	rank := 1.0
	for rows.Next() {
		var uri, snippet, srcScenes string
		var version int
		if err := rows.Scan(&uri, &snippet, &version, &srcScenes); err != nil {
			return nil, err
		}
		m := Match{URI: uri, Tier: tier, Rank: rank, Snippet: snippet, Version: version}
		if scenes, err := db.UnmarshalStringArray(srcScenes); err == nil {
			m.SourceScenes = scenes
		}
		out = append(out, m)
		rank++
	}
	return out, rows.Err()
}

func scanFTSMatches(rows *sql.Rows, tier string) ([]Match, error) {
	var out []Match
	rank := 1.0
	for rows.Next() {
		var uri, snippet string
		if err := rows.Scan(&uri, &snippet); err != nil {
			return nil, err
		}
		out = append(out, Match{URI: uri, Tier: tier, Rank: rank, Snippet: snippet})
		rank++
	}
	return out, rows.Err()
}
