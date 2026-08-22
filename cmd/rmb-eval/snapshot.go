package main

import (
	"database/sql"
	"fmt"

	"github.com/colinleefish/rmb-desktop/internal/recall/eval"
)

// snapshotMemories keeps every expected memory plus a deterministic stride
// sample of the rest as distractors, so the eval has both signal and noise.
func snapshotMemories(db *sql.DB, expected map[string]bool, stride int, fix *eval.Fixture) error {
	seen := map[string]bool{}
	add := func(uri string, q string, args ...any) error {
		if seen[uri] {
			return nil
		}
		seen[uri] = true
		var m eval.MemoryRow
		var sup sql.NullInt64
		err := db.QueryRow(q, args...).Scan(&m.ID, &m.URI, &m.Category, &m.Slug, &m.Version, &sup,
			&m.Abstract, &m.Body, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("expected URI %s not found in store (check golden.yaml)", uri)
			}
			return err
		}
		if sup.Valid {
			m.SupersededAt = &sup.Int64
		}
		fix.Memories = append(fix.Memories, m)
		return nil
	}

	// Stride sample of all memories (deterministic via rowid).
	rows, err := db.Query(`
		SELECT m.id, m.uri, m.category, COALESCE(m.slug,''), m.version, m.superseded_at,
		       COALESCE(m.abstract,''), COALESCE(m.body,''), m.created_at, m.updated_at
		FROM memories m
		WHERE m.rowid % ? = 0`, stride)
	if err != nil {
		return fmt.Errorf("select memories: %w", err)
	}
	for rows.Next() {
		var m eval.MemoryRow
		var sup sql.NullInt64
		if err := rows.Scan(&m.ID, &m.URI, &m.Category, &m.Slug, &m.Version, &sup,
			&m.Abstract, &m.Body, &m.CreatedAt, &m.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		if sup.Valid {
			m.SupersededAt = &sup.Int64
		}
		fix.Memories = append(fix.Memories, m)
		seen[m.URI] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	const q = `
		SELECT m.id, m.uri, m.category, COALESCE(m.slug,''), m.version, m.superseded_at,
		       COALESCE(m.abstract,''), COALESCE(m.body,''), m.created_at, m.updated_at
		FROM memories m WHERE m.uri = ?`
	for uri := range expected {
		if !isMemoryURI(uri) {
			continue
		}
		if err := add(uri, q, uri); err != nil {
			return err
		}
	}
	return nil
}

func snapshotScenes(db *sql.DB, expected map[string]bool, stride int, fix *eval.Fixture) error {
	seen := map[string]bool{}
	rows, err := db.Query(`
		SELECT s.id, s.session_id, COALESCE(s.display_name,''), COALESCE(s.abstract,''),
		       COALESCE(s.body,''), s.created_at, s.updated_at
		FROM scenes s WHERE s.rowid % ? = 0`, stride)
	if err != nil {
		return fmt.Errorf("select scenes: %w", err)
	}
	scan := func() (eval.SceneRow, error) {
		var s eval.SceneRow
		err := rows.Scan(&s.ID, &s.SessionID, &s.DisplayName, &s.Abstract, &s.Body, &s.CreatedAt, &s.UpdatedAt)
		return s, err
	}
	collect := func(s eval.SceneRow) { seen[s.ID] = true; fix.Scenes = append(fix.Scenes, s) }
	for rows.Next() {
		s, err := scan()
		if err != nil {
			rows.Close()
			return err
		}
		collect(s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	const q = `
		SELECT s.id, s.session_id, COALESCE(s.display_name,''), COALESCE(s.abstract,''),
		       COALESCE(s.body,''), s.created_at, s.updated_at
		FROM scenes s WHERE 'rmb://scenes/' || lower(s.id) = ?`
	for uri := range expected {
		if !isSceneURI(uri) {
			continue
		}
		var s eval.SceneRow
		if err := db.QueryRow(q, uri).Scan(&s.ID, &s.SessionID, &s.DisplayName, &s.Abstract, &s.Body, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return fmt.Errorf("expected scene %s: %w", uri, err)
		}
		if !seen[s.ID] {
			collect(s)
		}
	}
	return nil
}

func snapshotSkills(db *sql.DB, fix *eval.Fixture) error {
	rows, err := db.Query(`
		SELECT s.id, s.slug, s.uri, s.version, s.superseded_at, s.name,
		       s.description, s.fts_text, s.created_at, s.updated_at
		FROM skills s WHERE s.superseded_at IS NULL`)
	if err != nil {
		return fmt.Errorf("select skills: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k eval.SkillRow
		var sup sql.NullInt64
		if err := rows.Scan(&k.ID, &k.Slug, &k.URI, &k.Version, &sup, &k.Name,
			&k.Description, &k.FTSText, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return err
		}
		if sup.Valid {
			k.SupersededAt = &sup.Int64
		}
		fix.Skills = append(fix.Skills, k)
	}
	return rows.Err()
}

func isMemoryURI(uri string) bool {
	return len(uri) > 8 && (uri[:8] == "rmb://me" || uri[:8] == "rmb://ev" ||
		uri[:8] == "rmb://pr" || uri[:8] == "rmb://en" || uri[:8] == "rmb://ag")
}

func isSceneURI(uri string) bool {
	return len(uri) > 11 && uri[:11] == "rmb://scenes"
}
