package recallstats

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/uri"
)

// Stats holds per-URI recall counters for search hits, cat, and meta.
type Stats struct {
	URI            string  `json:"uri"`
	SearchCount    int64   `json:"search_count"`
	CatCount       int64   `json:"cat_count"`
	MetaCount      int64   `json:"meta_count"`
	LastSearchedAt *string `json:"last_searched_at,omitempty"`
	LastCatedAt    *string `json:"last_cated_at,omitempty"`
	LastMetaedAt   *string `json:"last_metaed_at,omitempty"`
	UpdatedAt      string  `json:"updated_at"`
}

type Service struct {
	db *sql.DB
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database}
}

// NormalizeURI maps a recall URI to the stats key for memories, scenes, and skills.
func NormalizeURI(raw string) (string, bool) {
	u, err := uri.Parse(raw)
	if err != nil {
		return "", false
	}
	switch u.Scope {
	case uri.ScopeProfile:
		return uri.BuildProfile(), true
	case uri.ScopeAgent:
		return uri.BuildAgent(), true
	case uri.ScopePrefs, uri.ScopeEntities, uri.ScopeEvents:
		return u.String(), true
	case uri.ScopeScenes:
		if len(u.Segments) == 0 {
			return "", false
		}
		return uri.BuildScene(u.Segments[0]), true
	case uri.ScopeSkills:
		if len(u.Segments) == 0 {
			return "", false
		}
		return uri.BuildSkill(u.Segments[0]), true
	default:
		return "", false
	}
}

func (s *Service) RecordSearch(ctx context.Context, uris []string) error {
	nowMS := time.Now().UTC().UnixMilli()
	for _, raw := range dedupeURIs(uris) {
		target, ok := NormalizeURI(raw)
		if !ok {
			target = strings.TrimSpace(raw)
			if target == "" {
				continue
			}
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, last_searched_at, updated_at)
			VALUES (?, 1, 0, 0, ?, ?)
			ON CONFLICT(uri) DO UPDATE SET
				search_count = search_count + 1,
				last_searched_at = excluded.last_searched_at,
				updated_at = excluded.updated_at`,
			target, nowMS, nowMS,
		)
		if err != nil {
			return fmt.Errorf("record search for %q: %w", target, err)
		}
	}
	return nil
}

func (s *Service) RecordCat(ctx context.Context, raw string) error {
	return s.recordKind(ctx, raw, "cat")
}

func (s *Service) RecordMeta(ctx context.Context, raw string) error {
	return s.recordKind(ctx, raw, "meta")
}

func (s *Service) recordKind(ctx context.Context, raw, kind string) error {
	target, ok := NormalizeURI(raw)
	if !ok {
		return nil
	}
	nowMS := time.Now().UTC().UnixMilli()
	var query string
	switch kind {
	case "cat":
		query = `
			INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, last_cated_at, updated_at)
			VALUES (?, 0, 1, 0, ?, ?)
			ON CONFLICT(uri) DO UPDATE SET
				cat_count = cat_count + 1,
				last_cated_at = excluded.last_cated_at,
				updated_at = excluded.updated_at`
	case "meta":
		query = `
			INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, last_metaed_at, updated_at)
			VALUES (?, 0, 0, 1, ?, ?)
			ON CONFLICT(uri) DO UPDATE SET
				meta_count = meta_count + 1,
				last_metaed_at = excluded.last_metaed_at,
				updated_at = excluded.updated_at`
	default:
		return fmt.Errorf("unknown recall kind %q", kind)
	}
	if _, err := s.db.ExecContext(ctx, query, target, nowMS, nowMS); err != nil {
		return fmt.Errorf("record %s for %q: %w", kind, target, err)
	}
	return nil
}

func (s *Service) RecordSearchAsync(uris []string) {
	if len(uris) == 0 {
		return
	}
	go func() {
		_ = s.RecordSearch(context.Background(), uris)
	}()
}

func (s *Service) RecordCatAsync(raw string) {
	go func() {
		_ = s.RecordCat(context.Background(), raw)
	}()
}

func (s *Service) RecordMetaAsync(raw string) {
	go func() {
		_ = s.RecordMeta(context.Background(), raw)
	}()
}

func (s *Service) BatchGet(ctx context.Context, uris []string) (map[string]Stats, error) {
	out := make(map[string]Stats, len(uris))
	keys := dedupeURIs(uris)
	if len(keys) == 0 {
		return out, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(keys)), ",")
	args := make([]any, len(keys))
	for i, key := range keys {
		args[i] = key
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT uri, search_count, cat_count, meta_count,
			last_searched_at, last_cated_at, last_metaed_at, updated_at
		FROM recall_stats
		WHERE uri IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("batch get recall stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row Stats
		var lastSearch, lastCat, lastMeta sql.NullInt64
		var updatedMS int64
		if err := rows.Scan(
			&row.URI, &row.SearchCount, &row.CatCount, &row.MetaCount,
			&lastSearch, &lastCat, &lastMeta, &updatedMS,
		); err != nil {
			return nil, err
		}
		row.LastSearchedAt = nullMSPtr(lastSearch)
		row.LastCatedAt = nullMSPtr(lastCat)
		row.LastMetaedAt = nullMSPtr(lastMeta)
		row.UpdatedAt = formatMS(updatedMS)
		out[row.URI] = row
	}
	return out, rows.Err()
}

func nullMSPtr(v sql.NullInt64) *string {
	if !v.Valid {
		return nil
	}
	s := formatMS(v.Int64)
	return &s
}

func formatMS(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func dedupeURIs(uris []string) []string {
	seen := make(map[string]struct{}, len(uris))
	out := make([]string, 0, len(uris))
	for _, raw := range uris {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out
}
