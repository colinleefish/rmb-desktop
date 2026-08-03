package browse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/skill"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

const (
	defaultListLimit = 300
	timeRFC3339      = time.RFC3339
)

var errNotFound = errors.New("not found")

// Service implements dashboard browse queries over SQLite.
type Service struct {
	db *sql.DB
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	var out Overview
	tables := []struct {
		dest *int64
		sql  string
	}{
		{&out.Counts.Sessions, `SELECT COUNT(*) FROM sessions`},
		{&out.Counts.Turns, `SELECT COUNT(*) FROM session_turns`},
		{&out.Counts.Atoms, `SELECT COUNT(*) FROM atoms`},
		{&out.Counts.Scenes, `SELECT COUNT(*) FROM scenes`},
		{&out.Counts.PipelineStates, `SELECT COUNT(*) FROM pipeline_state`},
		{&out.Counts.Corrections, `SELECT COUNT(*) FROM corrections WHERE retracted_at IS NULL`},
		{&out.Counts.Skills, `SELECT COUNT(*) FROM skills WHERE superseded_at IS NULL`},
	}
	for _, t := range tables {
		if err := s.db.QueryRowContext(ctx, t.sql).Scan(t.dest); err != nil {
			return Overview{}, fmt.Errorf("count: %w", err)
		}
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories WHERE superseded_at IS NULL`,
	).Scan(&out.Counts.Memories); err != nil {
		return Overview{}, fmt.Errorf("count memories: %w", err)
	}
	memStats, err := s.memoryCategoryOverview(ctx)
	if err != nil {
		return Overview{}, err
	}
	out.MemoryByCategory = memStats
	return out, nil
}

func (s *Service) memoryCategoryOverview(ctx context.Context) (MemoryCategoryOverview, error) {
	var stats MemoryCategoryOverview
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(version, 0)
		FROM memories
		WHERE category = 'profile' AND superseded_at IS NULL
		LIMIT 1`,
	).Scan(&stats.ProfileVersion); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return MemoryCategoryOverview{}, fmt.Errorf("profile version: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT category, COUNT(*)
		FROM memories
		WHERE superseded_at IS NULL AND category != 'profile'
		GROUP BY category`)
	if err != nil {
		return MemoryCategoryOverview{}, fmt.Errorf("memory categories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var category string
		var count int64
		if err := rows.Scan(&category, &count); err != nil {
			return MemoryCategoryOverview{}, err
		}
		switch category {
		case "events":
			stats.Events = count
		case "preferences":
			stats.Preferences = count
		case "entities":
			stats.Entities = count
		}
	}
	return stats, rows.Err()
}

func (s *Service) ListSessions(ctx context.Context, p ListParams) (Page[SessionRow], error) {
	limit := clampLimit(p.Limit)
	where, args := searchWhere(p.Query, []string{
		"s.session_key", "s.abstract", "s.source",
	})
	order := sessionOrder(p.Sort, p.Order)

	countSQL := `SELECT COUNT(*) FROM sessions s` + where
	var total int64
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return Page[SessionRow]{}, fmt.Errorf("count sessions: %w", err)
	}

	listSQL := `
		SELECT s.id, s.session_key, s.source, s.abstract, s.created_at, s.updated_at,
			COALESCE(ts.turn_count, 0), ts.last_turn_at
		FROM sessions s
		LEFT JOIN (
			SELECT session_id, COUNT(*) AS turn_count, MAX(created_at) AS last_turn_at
			FROM session_turns
			GROUP BY session_id
		) ts ON ts.session_id = s.id` + where + `
		ORDER BY ` + order + `
		LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), limit, p.Offset)

	rows, err := s.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return Page[SessionRow]{}, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionRow
	var ids []string
	for rows.Next() {
		var row SessionRow
		var abstract sql.NullString
		var source sql.NullString
		var createdMS, updatedMS int64
		var lastTurnMS sql.NullInt64
		if err := rows.Scan(
			&row.ID, &row.SessionKey, &source, &abstract, &createdMS, &updatedMS,
			&row.TurnCount, &lastTurnMS,
		); err != nil {
			return Page[SessionRow]{}, err
		}
		row.Status = "active"
		row.Source = nullStringPtr(source)
		row.Abstract = nullStringPtr(abstract)
		row.URI = uri.BuildSession(row.SessionKey)
		row.CreatedAt = formatMS(createdMS)
		row.UpdatedAt = formatMS(updatedMS)
		if lastTurnMS.Valid {
			v := formatMS(lastTurnMS.Int64)
			row.LastTurnAt = &v
		}
		ids = append(ids, row.ID)
		sessions = append(sessions, row)
	}
	if err := rows.Err(); err != nil {
		return Page[SessionRow]{}, err
	}

	summaries, err := s.loadSessionSummaries(ctx, ids)
	if err != nil {
		return Page[SessionRow]{}, err
	}
	for i := range sessions {
		if sum, ok := summaries[sessions[i].ID]; ok {
			sessions[i].AtomCount = sum.AtomCount
			sessions[i].SceneCount = sum.SceneCount
			sessions[i].T1Status = sum.T1Status
			sessions[i].T2Status = sum.T2Status
			sessions[i].T3Status = sum.T3Status
		}
	}

	return pageOf(sessions, total, limit, p.Offset), nil
}

type sessionSummary struct {
	AtomCount  int64
	SceneCount int64
	T1Status   string
	T2Status   string
	T3Status   string
}

func (s *Service) loadSessionSummaries(ctx context.Context, sessionIDs []string) (map[string]sessionSummary, error) {
	out := make(map[string]sessionSummary, len(sessionIDs))
	if len(sessionIDs) == 0 {
		return out, nil
	}
	placeholders, args := inClause(sessionIDs)

	atomSQL := fmt.Sprintf(
		`SELECT session_id, COUNT(*) FROM atoms WHERE session_id IN (%s) GROUP BY session_id`,
		placeholders,
	)
	if err := s.scanCounts(ctx, atomSQL, args, func(id string, n int64) {
		sum := out[id]
		sum.AtomCount = n
		out[id] = sum
	}); err != nil {
		return nil, err
	}

	sceneSQL := fmt.Sprintf(
		`SELECT session_id, COUNT(*) FROM scenes WHERE session_id IN (%s) GROUP BY session_id`,
		placeholders,
	)
	if err := s.scanCounts(ctx, sceneSQL, args, func(id string, n int64) {
		sum := out[id]
		sum.SceneCount = n
		out[id] = sum
	}); err != nil {
		return nil, err
	}

	pipeSQL := fmt.Sprintf(
		`SELECT session_id, l1_status, l2_status, l3_status FROM pipeline_state WHERE session_id IN (%s)`,
		placeholders,
	)
	rows, err := s.db.QueryContext(ctx, pipeSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, l1, l2, l3 string
		if err := rows.Scan(&id, &l1, &l2, &l3); err != nil {
			return nil, err
		}
		sum := out[id]
		sum.T1Status = l1
		sum.T2Status = l2
		sum.T3Status = l3
		out[id] = sum
	}
	return out, rows.Err()
}

func (s *Service) GetSession(ctx context.Context, sessionKey string) (SessionDetail, error) {
	sessionKey = strings.ToLower(strings.TrimSpace(sessionKey))
	var (
		id, key string
		abstract sql.NullString
		createdMS, updatedMS int64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, session_key, abstract, created_at, updated_at
		FROM sessions WHERE session_key = ?`, sessionKey,
	).Scan(&id, &key, &abstract, &createdMS, &updatedMS)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionDetail{}, errNotFound
	}
	if err != nil {
		return SessionDetail{}, fmt.Errorf("load session: %w", err)
	}

	turnRows, err := s.db.QueryContext(ctx, `
		SELECT id, messages_json, created_at
		FROM session_turns
		WHERE session_id = ?
		ORDER BY created_at ASC, id ASC`, id,
	)
	if err != nil {
		return SessionDetail{}, fmt.Errorf("load turns: %w", err)
	}
	defer turnRows.Close()

	var turns []TurnRow
	var lastTurnMS int64
	for i := 0; turnRows.Next(); i++ {
		var turn TurnRow
		var created int64
		if err := turnRows.Scan(&turn.ID, &turn.MessagesJSONL, &created); err != nil {
			return SessionDetail{}, err
		}
		turn.TurnIndex = i
		turn.URI = uri.BuildTurn(turn.ID)
		turn.CreatedAt = formatMS(created)
		turn.UpdatedAt = formatMS(created)
		turns = append(turns, turn)
		lastTurnMS = created
	}
	if err := turnRows.Err(); err != nil {
		return SessionDetail{}, err
	}

	var pipeline *PipelineStateJSON
	if ps, err := s.loadPipelineState(ctx, id); err == nil {
		pipeline = &ps
	} else if !errors.Is(err, errNotFound) {
		return SessionDetail{}, err
	}

	atoms, err := s.listAtomsForSession(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}
	scenes, err := s.listScenesForSession(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}

	summary := sessionSummary{
		AtomCount:  int64(len(atoms)),
		SceneCount: int64(len(scenes)),
	}
	if pipeline != nil {
		summary.T1Status = pipeline.T1Status
		summary.T2Status = pipeline.T2Status
		summary.T3Status = pipeline.T3Status
	}

	row := SessionRow{
		ID:         id,
		SessionKey: key,
		Status:     "active",
		Abstract:   nullStringPtr(abstract),
		TurnCount:  int64(len(turns)),
		AtomCount:  summary.AtomCount,
		SceneCount: summary.SceneCount,
		T1Status:   summary.T1Status,
		T2Status:   summary.T2Status,
		T3Status:   summary.T3Status,
		URI:        uri.BuildSession(key),
		CreatedAt:  formatMS(createdMS),
		UpdatedAt:  formatMS(updatedMS),
	}
	if len(turns) > 0 {
		v := formatMS(lastTurnMS)
		row.LastTurnAt = &v
	}

	return SessionDetail{
		Session:       row,
		Turns:         nonNilSlice(turns),
		PipelineState: pipeline,
		Atoms:         nonNilSlice(atoms),
		Scenes:        nonNilSlice(scenes),
	}, nil
}

func (s *Service) ListAtoms(ctx context.Context, p ListParams) (Page[AtomJSON], error) {
	return s.listAtoms(ctx, p, "")
}

func (s *Service) ListScenes(ctx context.Context, p ListParams) (Page[SceneJSON], error) {
	return s.listScenes(ctx, p, "")
}

func (s *Service) ListMemories(ctx context.Context, p ListParams) (Page[MemoryJSON], error) {
	limit := clampLimit(p.Limit)
	where, args := memoryListWhere(p)
	order := sortClause(map[string]string{
		"updated": "updated_at", "category": "category", "version": "version",
	}, "updated", p.Sort, p.Order)

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`+where, args...).Scan(&total); err != nil {
		return Page[MemoryJSON]{}, err
	}

	query := `
		SELECT id, uri, category, slug, version, abstract, body,
			source_scene_uris, source_correction_uris, created_at, updated_at
		FROM memories` + where + `
		ORDER BY ` + order + `
		LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), limit, p.Offset)
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return Page[MemoryJSON]{}, err
	}
	defer rows.Close()

	var items []MemoryJSON
	for rows.Next() {
		item, err := scanMemory(rows)
		if err != nil {
			return Page[MemoryJSON]{}, err
		}
		items = append(items, item)
	}
	return pageOf(items, total, limit, p.Offset), rows.Err()
}

func (s *Service) ListPipelineStates(ctx context.Context, p ListParams) (Page[PipelineStateRow], error) {
	limit := clampLimit(p.Limit)
	where := ""
	var args []any
	if q := strings.TrimSpace(p.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		where = ` WHERE lower(s.session_key) LIKE ? OR lower(ps.l1_status) LIKE ?
			OR lower(ps.l2_status) LIKE ? OR lower(ps.l3_status) LIKE ?`
		args = []any{like, like, like, like}
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pipeline_state ps
		JOIN sessions s ON s.id = ps.session_id`+where, args...,
	).Scan(&total); err != nil {
		return Page[PipelineStateRow]{}, err
	}

	query := `
		SELECT ps.session_id, ps.l1_status, ps.l2_status, ps.l3_status,
			ps.l1_advanced_at, ps.l2_advanced_at, ps.l3_advanced_at,
			ps.l1_turns_since_advanced, ps.warmup_threshold, ps.updated_at,
			s.session_key
		FROM pipeline_state ps
		JOIN sessions s ON s.id = ps.session_id` + where + `
		ORDER BY ps.updated_at DESC
		LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), limit, p.Offset)
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return Page[PipelineStateRow]{}, err
	}
	defer rows.Close()

	var items []PipelineStateRow
	for rows.Next() {
		var row PipelineStateRow
		var l1Adv, l2Adv, l3Adv sql.NullInt64
		var updatedMS int64
		if err := rows.Scan(
			&row.SessionID, &row.T1Status, &row.T2Status, &row.T3Status,
			&l1Adv, &l2Adv, &l3Adv,
			&row.T1TurnsSinceAdvanced, &row.WarmupThreshold, &updatedMS,
			&row.SessionKey,
		); err != nil {
			return Page[PipelineStateRow]{}, err
		}
		row.T1AdvancedAt = nullMSPtr(l1Adv)
		row.T2AdvancedAt = nullMSPtr(l2Adv)
		row.T3AdvancedAt = nullMSPtr(l3Adv)
		row.UpdatedAt = formatMS(updatedMS)
		row.SessionURI = uri.BuildSession(row.SessionKey)
		items = append(items, row)
	}
	return pageOf(items, total, limit, p.Offset), rows.Err()
}

func (s *Service) ListTasks(_ context.Context, p ListParams) (Page[map[string]any], error) {
	return pageOf([]map[string]any{}, 0, clampLimit(p.Limit), p.Offset), nil
}

func (s *Service) ListSkills(ctx context.Context, p ListParams) (Page[SkillRow], error) {
	limit := clampLimit(p.Limit)
	where, args := skillListWhere(p)
	order := sortClause(map[string]string{
		"updated": "updated_at", "name": "name", "version": "version",
	}, "updated", p.Sort, p.Order)

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM skills`+where, args...).Scan(&total); err != nil {
		return Page[SkillRow]{}, err
	}

	query := `
		SELECT slug, name, description, tags, uri, version, updated_at
		FROM skills` + where + `
		ORDER BY ` + order + `
		LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), limit, p.Offset)
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return Page[SkillRow]{}, err
	}
	defer rows.Close()

	var items []SkillRow
	for rows.Next() {
		item, err := scanSkill(rows)
		if err != nil {
			return Page[SkillRow]{}, err
		}
		items = append(items, item)
	}
	return pageOf(items, total, limit, p.Offset), rows.Err()
}

func (s *Service) GetSkill(ctx context.Context, slug string) (skill.Detail, error) {
	return skill.GetDetail(ctx, s.db, slug)
}

func scanSkill(rows *sql.Rows) (SkillRow, error) {
	var row SkillRow
	var tagsRaw string
	var updatedMS int64
	if err := rows.Scan(&row.Slug, &row.Name, &row.Description, &tagsRaw, &row.URI, &row.Version, &updatedMS); err != nil {
		return SkillRow{}, err
	}
	tags, err := db.UnmarshalStringArray(tagsRaw)
	if err != nil {
		return SkillRow{}, err
	}
	row.Tags = tags
	row.UpdatedAt = formatMS(updatedMS)
	return row, nil
}

func skillListWhere(p ListParams) (string, []any) {
	conds := []string{"superseded_at IS NULL"}
	var args []any
	if qWhere, qArgs := searchWhere(p.Query, []string{
		"name", "description", "slug", "uri",
	}); qWhere != "" {
		conds = append(conds, strings.TrimPrefix(qWhere, " WHERE "))
		args = append(args, qArgs...)
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func (s *Service) listAtoms(ctx context.Context, p ListParams, sessionID string) (Page[AtomJSON], error) {
	limit := clampLimit(p.Limit)
	where, args := searchWhere(p.Query, []string{
		"content", "category", "scene_name", "slug", "CAST(id AS TEXT)",
	})
	if sessionID != "" {
		if where == "" {
			where = " WHERE session_id = ?"
		} else {
			where += " AND session_id = ?"
		}
		args = append(args, sessionID)
	}
	order := sortClause(map[string]string{
		"created": "created_at", "category": "category", "priority": "priority",
	}, "created", p.Sort, p.Order)

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM atoms`+where, args...).Scan(&total); err != nil {
		return Page[AtomJSON]{}, err
	}

	query := `
		SELECT id, session_id, category, priority, scene_name, slug, content,
			source_turn_ids, created_at, updated_at
		FROM atoms` + where + `
		ORDER BY ` + order + `
		LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), limit, p.Offset)
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return Page[AtomJSON]{}, err
	}
	defer rows.Close()

	var items []AtomJSON
	for rows.Next() {
		item, err := scanAtom(rows)
		if err != nil {
			return Page[AtomJSON]{}, err
		}
		items = append(items, item)
	}
	return pageOf(items, total, limit, p.Offset), rows.Err()
}

func (s *Service) listAtomsForSession(ctx context.Context, sessionID string) ([]AtomJSON, error) {
	page, err := s.listAtoms(ctx, ListParams{Limit: defaultListLimit}, sessionID)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (s *Service) listScenes(ctx context.Context, p ListParams, sessionID string) (Page[SceneJSON], error) {
	limit := clampLimit(p.Limit)
	where, args := searchWhere(p.Query, []string{
		"display_name", "abstract", "body", "CAST(id AS TEXT)",
	})
	if sessionID != "" {
		if where == "" {
			where = " WHERE session_id = ?"
		} else {
			where += " AND session_id = ?"
		}
		args = append(args, sessionID)
	}
	order := sortClause(map[string]string{
		"updated": "updated_at", "created": "created_at",
	}, "updated", p.Sort, p.Order)

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM scenes`+where, args...).Scan(&total); err != nil {
		return Page[SceneJSON]{}, err
	}

	query := `
		SELECT id, session_id, display_name, abstract, body, source_atoms, created_at, updated_at
		FROM scenes` + where + `
		ORDER BY ` + order + `
		LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), limit, p.Offset)
	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return Page[SceneJSON]{}, err
	}
	defer rows.Close()

	var items []SceneJSON
	for rows.Next() {
		item, err := scanScene(rows)
		if err != nil {
			return Page[SceneJSON]{}, err
		}
		items = append(items, item)
	}
	return pageOf(items, total, limit, p.Offset), rows.Err()
}

func (s *Service) listScenesForSession(ctx context.Context, sessionID string) ([]SceneJSON, error) {
	page, err := s.listScenes(ctx, ListParams{Limit: defaultListLimit}, sessionID)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (s *Service) loadPipelineState(ctx context.Context, sessionID string) (PipelineStateJSON, error) {
	var ps PipelineStateJSON
	var l1Adv, l2Adv, l3Adv sql.NullInt64
	var updatedMS int64
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, l1_status, l2_status, l3_status,
			l1_advanced_at, l2_advanced_at, l3_advanced_at,
			l1_turns_since_advanced, warmup_threshold, updated_at
		FROM pipeline_state WHERE session_id = ?`, sessionID,
	).Scan(
		&ps.SessionID, &ps.T1Status, &ps.T2Status, &ps.T3Status,
		&l1Adv, &l2Adv, &l3Adv,
		&ps.T1TurnsSinceAdvanced, &ps.WarmupThreshold, &updatedMS,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PipelineStateJSON{}, errNotFound
	}
	if err != nil {
		return PipelineStateJSON{}, err
	}
	ps.T1AdvancedAt = nullMSPtr(l1Adv)
	ps.T2AdvancedAt = nullMSPtr(l2Adv)
	ps.T3AdvancedAt = nullMSPtr(l3Adv)
	ps.UpdatedAt = formatMS(updatedMS)
	return ps, nil
}

func scanAtom(rows *sql.Rows) (AtomJSON, error) {
	var a AtomJSON
	var sceneName, slug sql.NullString
	var sourceRaw string
	var createdMS, updatedMS int64
	if err := rows.Scan(
		&a.ID, &a.SessionID, &a.Category, &a.Priority, &sceneName, &slug,
		&a.Content, &sourceRaw, &createdMS, &updatedMS,
	); err != nil {
		return AtomJSON{}, err
	}
	a.SceneName = nullStringPtr(sceneName)
	a.Slug = nullStringPtr(slug)
	ids, err := db.UnmarshalStringArray(sourceRaw)
	if err != nil {
		return AtomJSON{}, err
	}
	a.SourceTurnIDs = ids
	a.CreatedAt = formatMS(createdMS)
	a.UpdatedAt = formatMS(updatedMS)
	a.URI = uri.BuildAtom(a.ID)
	return a, nil
}

func scanScene(rows *sql.Rows) (SceneJSON, error) {
	var sc SceneJSON
	var displayName, abstract, body sql.NullString
	var sourceRaw string
	var createdMS, updatedMS int64
	if err := rows.Scan(
		&sc.ID, &sc.SessionID, &displayName, &abstract, &body,
		&sourceRaw, &createdMS, &updatedMS,
	); err != nil {
		return SceneJSON{}, err
	}
	sc.DisplayName = nullStringPtr(displayName)
	sc.Abstract = nullStringPtr(abstract)
	sc.Body = nullStringPtr(body)
	ids, err := db.UnmarshalStringArray(sourceRaw)
	if err != nil {
		return SceneJSON{}, err
	}
	sc.SourceAtoms = ids
	sc.CreatedAt = formatMS(createdMS)
	sc.UpdatedAt = formatMS(updatedMS)
	sc.URI = uri.BuildScene(sc.ID)
	return sc, nil
}

func scanMemory(rows *sql.Rows) (MemoryJSON, error) {
	var m MemoryJSON
	var slug, abstract, body sql.NullString
	var scenesRaw, correctionsRaw string
	var createdMS, updatedMS int64
	if err := rows.Scan(
		&m.ID, &m.URI, &m.Category, &slug, &m.Version, &abstract, &body,
		&scenesRaw, &correctionsRaw, &createdMS, &updatedMS,
	); err != nil {
		return MemoryJSON{}, err
	}
	m.Slug = nullStringPtr(slug)
	m.Abstract = nullStringPtr(abstract)
	m.Body = nullStringPtr(body)
	scenes, err := db.UnmarshalStringArray(scenesRaw)
	if err != nil {
		return MemoryJSON{}, err
	}
	corrections, err := db.UnmarshalStringArray(correctionsRaw)
	if err != nil {
		return MemoryJSON{}, err
	}
	m.SourceSceneURIs = scenes
	m.SourceCorrectionURIs = corrections
	m.CreatedAt = formatMS(createdMS)
	m.UpdatedAt = formatMS(updatedMS)
	return m, nil
}

func (s *Service) scanCounts(ctx context.Context, sql string, args []any, fn func(string, int64)) error {
	rows, err := s.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			return err
		}
		fn(id, n)
	}
	return rows.Err()
}

func memoryListWhere(p ListParams) (string, []any) {
	conds := []string{"superseded_at IS NULL"}
	var args []any

	if cat := strings.TrimSpace(p.Category); cat != "" {
		conds = append(conds, "category = ?")
		args = append(args, cat)
	}
	if qWhere, qArgs := searchWhere(p.Query, []string{
		"abstract", "body", "slug", "uri", "category",
	}); qWhere != "" {
		conds = append(conds, strings.TrimPrefix(qWhere, " WHERE "))
		args = append(args, qArgs...)
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func searchWhere(query string, cols []string) (string, []any) {
	query = strings.TrimSpace(query)
	if query == "" || len(cols) == 0 {
		return "", nil
	}
	like := "%" + strings.ToLower(query) + "%"
	parts := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, col := range cols {
		parts[i] = "lower(" + col + ") LIKE ?"
		args[i] = like
	}
	return " WHERE " + strings.Join(parts, " OR "), args
}

func sessionOrder(sort, order string) string {
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	switch sort {
	case "created":
		return "s.created_at " + dir
	case "status":
		return "s.session_key " + dir
	default:
		return "COALESCE(ts.last_turn_at, s.created_at) " + dir
	}
}

func sortClause(allowed map[string]string, defaultKey, sort, order string) string {
	col, ok := allowed[sort]
	if !ok {
		col = allowed[defaultKey]
	}
	dir := "DESC"
	if strings.EqualFold(order, "asc") {
		dir = "ASC"
	}
	return col + " " + dir
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func inClause(ids []string) (string, []any) {
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return strings.Join(ph, ","), args
}

func formatMS(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(timeRFC3339)
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func nullMSPtr(v sql.NullInt64) *string {
	if !v.Valid {
		return nil
	}
	s := formatMS(v.Int64)
	return &s
}

// IsNotFound reports whether err is a missing browse entity.
func IsNotFound(err error) bool {
	return errors.Is(err, errNotFound)
}

func pageOf[T any](items []T, total int64, limit, offset int) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Total: total, Limit: limit, Offset: offset}
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
