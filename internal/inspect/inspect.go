package inspect

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/agentmemory"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/skill"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

type Service struct {
	db *sql.DB
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

func (s *Service) Cat(ctx context.Context, raw string, w io.Writer) error {
	u, err := uri.Parse(raw)
	if err != nil {
		return err
	}
	switch u.Scope {
	case uri.ScopeRoot:
		_, err := fmt.Fprintln(w, "rmb root — use `rmb tree rmb://` to list scopes")
		return err
	case uri.ScopeAgent:
		_, err := io.WriteString(w, agentmemory.AgentGuideBody())
		return err
	case uri.ScopeProfile, uri.ScopePrefs, uri.ScopeEntities, uri.ScopeEvents:
		return s.catMemoryByURI(ctx, u.String(), w, u.Scope)
	case uri.ScopeScenes:
		if len(u.Segments) == 0 {
			return fmt.Errorf("scene id required; use `tree %s`", u.String())
		}
		return s.catScene(ctx, u.Segments[0], w)
	case uri.ScopeAtoms:
		if len(u.Segments) == 0 {
			return fmt.Errorf("atom id required; use `tree %s`", u.String())
		}
		return s.catAtom(ctx, u.Segments[0], w)
	case uri.ScopeTurns:
		if len(u.Segments) == 0 {
			return fmt.Errorf("turn id required; use `tree %s`", u.String())
		}
		return s.catTurn(ctx, u.Segments[0], w)
	case uri.ScopeSessions:
		return s.catSession(ctx, u, w)
	case uri.ScopeSkills:
		return s.catSkill(ctx, u, w)
	default:
		return fmt.Errorf("unsupported scope %q", u.Scope)
	}
}

func (s *Service) Tree(ctx context.Context, raw string, w io.Writer) error {
	u, err := uri.Parse(raw)
	if err != nil {
		return err
	}
	if u.IsRoot() && !u.IsContainer() {
		return s.treeRoot(w)
	}
	switch u.Scope {
	case uri.ScopeSessions:
		return s.treeSession(ctx, u, w)
	case uri.ScopeScenes, uri.ScopeAtoms, uri.ScopeTurns, uri.ScopePrefs, uri.ScopeEntities, uri.ScopeEvents, uri.ScopeProfile, uri.ScopeAgent:
		return s.treeScope(ctx, u, w)
	case uri.ScopeSkills:
		return s.treeSkill(ctx, u, w)
	default:
		return fmt.Errorf("unsupported tree prefix %q", u.String())
	}
}

func (s *Service) Meta(ctx context.Context, raw string, w io.Writer) error {
	u, err := uri.Parse(raw)
	if err != nil {
		return err
	}
	var payload any
	switch u.Scope {
	case uri.ScopeProfile:
		payload, err = s.metaMemory(ctx, uri.BuildProfile())
	case uri.ScopeAgent:
		payload = s.metaAgent()
	case uri.ScopePrefs, uri.ScopeEntities, uri.ScopeEvents:
		payload, err = s.metaMemory(ctx, u.String())
	case uri.ScopeScenes:
		payload, err = s.metaScene(ctx, u)
	case uri.ScopeAtoms:
		payload, err = s.metaAtom(ctx, u)
	case uri.ScopeTurns:
		payload, err = s.metaTurn(ctx, u)
	case uri.ScopeSessions:
		payload, err = s.metaSession(ctx, u)
	case uri.ScopeSkills:
		payload, err = s.metaSkill(ctx, u)
	default:
		return fmt.Errorf("unsupported meta uri %q", u.String())
	}
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func (s *Service) treeRoot(w io.Writer) error {
	scopes := []string{
		uri.BuildProfile(),
		uri.BuildAgent(),
		"rmb://sessions/",
		"rmb://turns/",
		"rmb://atoms/",
		"rmb://scenes/",
		"rmb://preferences/",
		"rmb://entities/",
		"rmb://events/",
		"rmb://skills/",
	}
	for _, line := range scopes {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) catMemoryByURI(ctx context.Context, target string, w io.Writer, scope string) error {
	if scope == uri.ScopeProfile {
		target = uri.BuildProfile()
	}
	return s.catMemory(ctx, target, w)
}

func (s *Service) catMemory(ctx context.Context, target string, w io.Writer) error {
	var body sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT body FROM memories WHERE uri = ? AND superseded_at IS NULL`, target,
	).Scan(&body)
	if err == sql.ErrNoRows {
		return fmt.Errorf("memory not found: %s", target)
	}
	if err != nil {
		return err
	}
	text := ""
	if body.Valid {
		text = body.String
	}
	_, err = io.WriteString(w, text)
	return err
}

func (s *Service) catTurn(ctx context.Context, turnID string, w io.Writer) error {
	var messages string
	err := s.db.QueryRowContext(ctx, `SELECT messages_json FROM session_turns WHERE id = ?`, turnID).Scan(&messages)
	if err != nil {
		return fmt.Errorf("load turn: %w", err)
	}
	_, err = io.WriteString(w, messages)
	return err
}

func (s *Service) catAtom(ctx context.Context, atomID string, w io.Writer) error {
	var content string
	err := s.db.QueryRowContext(ctx, `SELECT content FROM atoms WHERE id = ?`, atomID).Scan(&content)
	if err != nil {
		return fmt.Errorf("load atom: %w", err)
	}
	_, err = io.WriteString(w, content)
	return err
}

func (s *Service) catScene(ctx context.Context, sceneID string, w io.Writer) error {
	var body sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT body FROM scenes WHERE id = ?`, sceneID).Scan(&body)
	if err != nil {
		return fmt.Errorf("load scene: %w", err)
	}
	text := ""
	if body.Valid {
		text = body.String
	}
	_, err = io.WriteString(w, text)
	return err
}

func (s *Service) catSession(ctx context.Context, u uri.URI, w io.Writer) error {
	if len(u.Segments) == 0 {
		return errors.New("session key required")
	}
	sessionKey := strings.ToLower(u.Segments[0])
	var abstract sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT abstract FROM sessions WHERE session_key = ?`, sessionKey).Scan(&abstract)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	text := ""
	if abstract.Valid {
		text = abstract.String
	}
	_, err = io.WriteString(w, text)
	return err
}

func (s *Service) treeSession(ctx context.Context, u uri.URI, w io.Writer) error {
	if len(u.Segments) == 0 {
		rows, err := s.db.QueryContext(ctx, `SELECT session_key FROM sessions ORDER BY updated_at DESC LIMIT 200`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, uri.BuildSession(key)+"/"); err != nil {
				return err
			}
		}
		return rows.Err()
	}

	sessionKey := strings.ToLower(u.Segments[0])
	var sessionID string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM sessions WHERE session_key = ?`, sessionKey).Scan(&sessionID); err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	if len(u.Segments) == 1 && u.IsContainer() {
		turnRows, err := s.db.QueryContext(ctx, `
			SELECT id FROM session_turns WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
		if err != nil {
			return err
		}
		for turnRows.Next() {
			var id string
			if err := turnRows.Scan(&id); err != nil {
				turnRows.Close()
				return err
			}
			if _, err := fmt.Fprintln(w, uri.BuildTurn(id)); err != nil {
				turnRows.Close()
				return err
			}
		}
		turnRows.Close()

		atomRows, err := s.db.QueryContext(ctx, `
			SELECT id FROM atoms WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
		if err != nil {
			return err
		}
		for atomRows.Next() {
			var id string
			if err := atomRows.Scan(&id); err != nil {
				atomRows.Close()
				return err
			}
			if _, err := fmt.Fprintln(w, uri.BuildAtom(id)); err != nil {
				atomRows.Close()
				return err
			}
		}
		return atomRows.Close()
	}
	return fmt.Errorf("tree not supported for %q", u.String())
}

func (s *Service) treeScope(ctx context.Context, u uri.URI, w io.Writer) error {
	switch u.Scope {
	case uri.ScopeProfile:
		var count int
		_ = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM memories WHERE category = 'profile' AND superseded_at IS NULL`).Scan(&count)
		if count > 0 {
			_, err := fmt.Fprintln(w, uri.BuildProfile())
			return err
		}
		return nil
	case uri.ScopeAgent:
		_, err := fmt.Fprintln(w, uri.BuildAgent())
		return err
	case uri.ScopeScenes:
		q := `SELECT id FROM scenes ORDER BY updated_at DESC LIMIT 200`
		args := []any{}
		if len(u.Segments) == 1 {
			q = `SELECT id FROM scenes WHERE id = ?`
			args = append(args, u.Segments[0])
		}
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, uri.BuildScene(id)); err != nil {
				return err
			}
		}
		return rows.Err()
	case uri.ScopeAtoms:
		q := `SELECT id FROM atoms ORDER BY created_at ASC LIMIT 200`
		args := []any{}
		if len(u.Segments) == 1 {
			q = `SELECT id FROM atoms WHERE id = ?`
			args = append(args, u.Segments[0])
		}
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, uri.BuildAtom(id)); err != nil {
				return err
			}
		}
		return rows.Err()
	case uri.ScopeTurns:
		q := `SELECT id FROM session_turns ORDER BY created_at ASC LIMIT 200`
		args := []any{}
		if len(u.Segments) == 1 {
			q = `SELECT id FROM session_turns WHERE id = ?`
			args = append(args, u.Segments[0])
		}
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, uri.BuildTurn(id)); err != nil {
				return err
			}
		}
		return rows.Err()
	default:
		q := `SELECT uri FROM memories WHERE category = ? AND superseded_at IS NULL ORDER BY updated_at DESC LIMIT 200`
		args := []any{u.Scope}
		if len(u.Segments) == 1 {
			q += ` AND uri = ?`
			args = append(args, uri.BuildMemory(u.Scope, u.Segments[0]))
		}
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var memURI string
			if err := rows.Scan(&memURI); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(w, memURI); err != nil {
				return err
			}
		}
		return rows.Err()
	}
}

func (s *Service) metaMemory(ctx context.Context, target string) (map[string]any, error) {
	var uriVal, category string
	var slug, abstract, body sql.NullString
	var version int
	var sourceScenes string
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, `
		SELECT uri, category, slug, abstract, body, source_scene_uris, version, created_at, updated_at
		FROM memories WHERE uri = ? AND superseded_at IS NULL`, target,
	).Scan(&uriVal, &category, &slug, &abstract, &body, &sourceScenes, &version, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("load memory: %w", err)
	}
	scenes, _ := db.UnmarshalStringArray(sourceScenes)
	meta := map[string]any{
		"uri":               uriVal,
		"version":           version,
		"category":          category,
		"abstract":          nullStr(abstract),
		"body":              nullStr(body),
		"source_scene_uris": scenes,
		"created_at":        createdAt,
		"updated_at":        updatedAt,
	}
	if slug.Valid {
		meta["slug"] = slug.String
	}
	return meta, nil
}

// metaAgent synthesizes the rmb://agent meta payload from the embedded bundle.
// The agent guide is curated documentation, not a distilled memory, so it has
// no version or timestamps — only uri/category/abstract/body.
func (s *Service) metaAgent() map[string]any {
	return map[string]any{
		"uri":               uri.BuildAgent(),
		"category":          uri.ScopeAgent,
		"abstract":          agentmemory.AgentGuideAbstract(),
		"body":              agentmemory.AgentGuideBody(),
		"source_scene_uris": []string{},
	}
}

func (s *Service) metaScene(ctx context.Context, u uri.URI) (map[string]any, error) {
	if len(u.Segments) != 1 {
		return nil, fmt.Errorf("scene id required")
	}
	var id, sessionID string
	var displayName, abstract, body sql.NullString
	var sourceAtoms string
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, session_id, display_name, abstract, body, source_atoms, created_at, updated_at
		FROM scenes WHERE id = ?`, u.Segments[0],
	).Scan(&id, &sessionID, &displayName, &abstract, &body, &sourceAtoms, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("load scene: %w", err)
	}
	atoms, _ := db.UnmarshalStringArray(sourceAtoms)
	return map[string]any{
		"uri":          uri.BuildScene(id),
		"id":           id,
		"session_id":   sessionID,
		"display_name": nullStr(displayName),
		"abstract":     nullStr(abstract),
		"body":         nullStr(body),
		"source_atoms": atoms,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

func (s *Service) metaTurn(ctx context.Context, u uri.URI) (map[string]any, error) {
	if len(u.Segments) != 1 {
		return nil, fmt.Errorf("turn id required")
	}
	var sessionID string
	var createdAt int64
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, created_at FROM session_turns WHERE id = ?`, u.Segments[0],
	).Scan(&sessionID, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("load turn: %w", err)
	}
	return map[string]any{
		"uri":        uri.BuildTurn(u.Segments[0]),
		"session_id": sessionID,
		"created_at": createdAt,
	}, nil
}

func (s *Service) metaAtom(ctx context.Context, u uri.URI) (map[string]any, error) {
	if len(u.Segments) != 1 {
		return nil, fmt.Errorf("atom id required")
	}
	var id, sessionID, category, content, sourceTurns string
	var priority int
	var sceneName, slug sql.NullString
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at
		FROM atoms WHERE id = ?`, u.Segments[0],
	).Scan(&id, &sessionID, &category, &priority, &sceneName, &slug, &content, &sourceTurns, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("load atom: %w", err)
	}
	turns, _ := db.UnmarshalStringArray(sourceTurns)
	meta := map[string]any{
		"uri":             uri.BuildAtom(id),
		"id":              id,
		"session_id":      sessionID,
		"category":        category,
		"priority":        priority,
		"content":         content,
		"source_turn_ids": turns,
		"created_at":      createdAt,
		"updated_at":      updatedAt,
	}
	if sceneName.Valid {
		meta["scene_name"] = sceneName.String
	}
	if slug.Valid {
		meta["slug"] = slug.String
	}
	return meta, nil
}

func (s *Service) metaSession(ctx context.Context, u uri.URI) (map[string]any, error) {
	if len(u.Segments) == 0 {
		return nil, errors.New("session key required")
	}
	sessionKey := strings.ToLower(u.Segments[0])
	var abstract sql.NullString
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx, `
		SELECT abstract, created_at, updated_at FROM sessions WHERE session_key = ?`, sessionKey,
	).Scan(&abstract, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	return map[string]any{
		"uri":         uri.BuildSession(sessionKey),
		"session_key": sessionKey,
		"abstract":    nullStr(abstract),
		"created_at":  createdAt,
		"updated_at":  updatedAt,
	}, nil
}

func nullStr(v sql.NullString) any {
	if v.Valid {
		return v.String
	}
	return nil
}

func (s *Service) catSkill(ctx context.Context, u uri.URI, w io.Writer) error {
	if len(u.Segments) == 0 {
		return fmt.Errorf("skill name required; use `tree %s` to list skills", u.String())
	}
	slug := u.Segments[0]
	relPath := skill.ManifestPath
	if len(u.Segments) > 1 {
		relPath = strings.Join(u.Segments[1:], "/")
	}
	text, err := skill.ReadFile(ctx, s.db, slug, relPath)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, text)
	return err
}

func (s *Service) treeSkill(ctx context.Context, u uri.URI, w io.Writer) error {
	if len(u.Segments) == 0 {
		catalog, err := skill.ListCatalog(ctx, s.db)
		if err != nil {
			return err
		}
		for _, e := range catalog {
			parsed, err := uri.Parse(e.URI)
			if err != nil || len(parsed.Segments) < 1 {
				continue
			}
			line := uri.BuildSkill(parsed.Segments[0]) + "/"
			desc := e.Description
			if len(e.Tags) > 0 {
				desc = "[" + strings.Join(e.Tags, ", ") + "] " + desc
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\n", line, desc); err != nil {
				return err
			}
		}
		return nil
	}
	slug := u.Segments[0]
	prefix := ""
	if len(u.Segments) > 1 {
		prefix = strings.Join(u.Segments[1:], "/")
	}
	children, err := skill.ListTreeChildren(ctx, s.db, slug, prefix)
	if err != nil {
		return err
	}
	for _, line := range children {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) metaSkill(ctx context.Context, u uri.URI) (map[string]any, error) {
	if len(u.Segments) == 0 {
		return nil, fmt.Errorf("skill name required")
	}
	slug := u.Segments[0]
	row, err := skill.LoadActive(ctx, s.db, slug)
	if err != nil {
		return nil, fmt.Errorf("load skill: %w", err)
	}
	files, err := skill.LoadFiles(ctx, s.db, row.ID)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.RelPath)
	}
	sort.Strings(paths)
	return map[string]any{
		"uri":           row.URI,
		"slug":          row.Slug,
		"name":          row.Name,
		"description":   row.Description,
		"tags":          append([]string(nil), row.Tags...),
		"version":       row.Version,
		"bundle_sha256": row.BundleSHA256,
		"files":         paths,
		"created_at":    row.CreatedAt,
		"updated_at":    row.UpdatedAt,
	}, nil
}
