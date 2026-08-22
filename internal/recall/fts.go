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

// EscapeFTSQueryAny quotes each field of the query as a phrase and joins
// them with OR — a recall-oriented variant of EscapeFTSQuery used where any
// shared term makes a memory a candidate and bm25 ranking does the sorting
// (L3 event linking, P2.2 / issue #28).
func EscapeFTSQueryAny(query string) string {
	parts := strings.Fields(strings.TrimSpace(query))
	if len(parts) == 0 {
		return `""`
	}
	for i, p := range parts {
		p = strings.ReplaceAll(p, `"`, `""`)
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, " OR ")
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
		WHERE memories_fts MATCH ? AND m.superseded_at IS NULL AND m.archived_at IS NULL`+windowClause+`
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

// FTSEventLinks returns the top-k active EVENT memories matching query by
// BM25, excluding excludeURI (the event being distilled). It backs L3
// retrieve-then-link (P2.2, issue #28): the distill prompt injects these so a
// resolution event can link the earlier problem event it resolves. The query
// is OR-joined per token (EscapeFTSQueryAny); bm25 sorts the candidates.
func FTSEventLinks(ctx context.Context, db *sql.DB, query, excludeURI string, k int) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	rows, err := db.QueryContext(ctx, `
		SELECT m.uri,
		       COALESCE(substr(COALESCE(NULLIF(TRIM(m.abstract), ''), m.body), 1, 160), '') AS snippet,
		       m.version,
		       m.source_scene_uris
		FROM memories m
		INNER JOIN memories_fts fts ON fts.rowid = m.rowid
		WHERE memories_fts MATCH ? AND m.superseded_at IS NULL
		  AND m.category = 'events' AND m.uri <> ?
		ORDER BY bm25(memories_fts)
		LIMIT ?`, EscapeFTSQueryAny(query), excludeURI, k)
	if err != nil {
		return nil, fmt.Errorf("fts event links: %w", err)
	}
	defer rows.Close()
	return scanFTSMemoryMatches(rows, "events")
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

// FTSAtoms returns atom results (the raw per-fact evidence tier) ranked by
// BM25. Atoms are an explicit --scope=atom tier (never default). Each hit is
// annotated with its owning scene URI and session URI so agents can drill
// down to the evidence without re-distillation (plan §5 P1.1, C6).
func FTSAtoms(ctx context.Context, db *sql.DB, query string, k int, tw TimeWindow) ([]Match, error) {
	if k <= 0 {
		k = 5
	}
	windowClause, windowArgs := tw.Clause("a.updated_at")
	rows, err := db.QueryContext(ctx, `
		SELECT 'rmb://atoms/' || lower(a.id) AS uri,
		       COALESCE(substr(a.content, 1, 160), '') AS snippet,
		       COALESCE((SELECT 'rmb://scenes/' || lower(s.id)
		                 FROM scenes s
		                 WHERE s.source_atoms LIKE '%"' || a.id || '"%'
		                 LIMIT 1), '') AS scene_uri,
		       'rmb://sessions/' || lower(a.session_id) AS session_uri
		FROM atoms a
		INNER JOIN atoms_fts fts ON fts.rowid = a.rowid
		WHERE atoms_fts MATCH ?`+windowClause+`
		ORDER BY bm25(atoms_fts)
		LIMIT ?`, prependArgs(EscapeFTSQuery(query), append(windowArgs, any(k))...)...)
	if err != nil {
		return nil, fmt.Errorf("fts atoms: %w", err)
	}
	defer rows.Close()
	return scanAtomMatches(rows, "atoms")
}

// scanAtomMatches reads an atom-tier row (uri, snippet, scene_uri,
// session_uri) and appends drill-down annotations to the snippet.
func scanAtomMatches(rows *sql.Rows, tier string) ([]Match, error) {
	var out []Match
	rank := 1.0
	for rows.Next() {
		var uri, snippet, sceneURI, sessionURI string
		if err := rows.Scan(&uri, &snippet, &sceneURI, &sessionURI); err != nil {
			return nil, err
		}
		out = append(out, annotateAtom(Match{URI: uri, Tier: tier, Rank: rank, Snippet: snippet}, sceneURI, sessionURI))
		rank++
	}
	return out, rows.Err()
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
