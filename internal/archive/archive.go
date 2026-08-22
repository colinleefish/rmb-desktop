// Package archive implements reversible, doctor-proposed archival of cold
// memories (plan §10, issue #32 / P3.3, D2=90d).
//
// Archival is "growth without forgetting": cold memories leave the default
// search candidate pool but stay cat-able by direct uri and are restorable in
// one command. Nothing auto-deletes. Evidence tiers (turns/atoms/scenes/
// skills) are exempt forever — the recoverable-evidence invariant.
package archive

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ColdWindowDays is the archival threshold (one quarter). A memory is a
// candidate when it has had no qualifying use (cat/meta heat) in this window
// AND its heat has decayed to ~0.
const ColdWindowDays = 90

// HeatEpsilon is the "heat ≈ 0" cutoff for archival candidacy. Uses a small
// positive value so a single stale cat from weeks ago does not hold a memory
// hostage, while still excluding anything meaningfully used.
const HeatEpsilon = 0.05

// profileURI is never archived: it is the singleton about who the user is.
const profileURI = "rmb://profile"

// Candidate is one cold memory proposed for archival (the doctor's reviewable
// list).
type Candidate struct {
	URI       string  `json:"uri"`
	Category  string  `json:"category"`
	Slug      string  `json:"slug,omitempty"`
	Abstract  string  `json:"abstract,omitempty"`
	Version   int     `json:"version"`
	Heat      float64 `json:"heat"`
	LastUseAt *int64  `json:"last_use_at,omitempty"`
	UpdatedAt int64   `json:"updated_at"`
}

// Service proposes and applies archival of the memories table.
type Service struct {
	db  *sql.DB
	now func() time.Time
}

// NewService returns an archive Service over the given database.
func NewService(database *sql.DB) *Service {
	return &Service{db: database, now: func() time.Time { return time.Now().UTC() }}
}

// SetClock overrides the service clock (unit tests). nil restores time.Now.
func (s *Service) SetClock(now func() time.Time) {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	s.now = now
}

// Candidates returns the cold, active memories that meet the archival policy
// (the doctor's proposed list / --dry-run):
//
//   - active (superseded_at IS NULL) — superseded chains are not current
//   - not yet archived (archived_at IS NULL)
//   - not the profile singleton
//   - heat ≈ 0 (COALESCE heat <= HeatEpsilon)
//   - no qualifying use (cat/meta) within the cold window (last_use_at NULL
//     or older than `days`)
//   - not correction-linked (no live correction targets this uri)
//
// `days` defaults to ColdWindowDays when <= 0.
func (s *Service) Candidates(ctx context.Context, days int) ([]Candidate, error) {
	if days <= 0 {
		days = ColdWindowDays
	}
	cutoff := s.now().Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.uri, m.category, COALESCE(m.slug, ''), COALESCE(m.abstract, ''),
		       m.version, m.updated_at,
		       COALESCE(r.heat, 0), r.last_use_at
		FROM memories m
		LEFT JOIN recall_stats r ON r.uri = m.uri
		WHERE m.superseded_at IS NULL
		  AND m.archived_at IS NULL
		  AND m.uri <> ?
		  AND COALESCE(r.heat, 0) <= ?
		  AND (r.last_use_at IS NULL OR r.last_use_at < ?)
		  AND NOT EXISTS (
		      SELECT 1 FROM corrections c
		      WHERE c.retracted_at IS NULL
		        AND c.target_uris LIKE '%"' || m.uri || '"%'
		  )
		ORDER BY m.updated_at ASC`, profileURI, HeatEpsilon, cutoff)
	if err != nil {
		return nil, fmt.Errorf("archive candidates: %w", err)
	}
	defer rows.Close()

	var out []Candidate
	for rows.Next() {
		var c Candidate
		var slug, abstract string
		var lastUse sql.NullInt64
		if err := rows.Scan(&c.URI, &c.Category, &slug, &abstract, &c.Version, &c.UpdatedAt,
			&c.Heat, &lastUse); err != nil {
			return nil, err
		}
		c.Slug = slug
		c.Abstract = abstract
		if lastUse.Valid {
			c.LastUseAt = &lastUse.Int64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Apply archives the given URIs (must be active rows). When uris is empty it
// archives every current candidate (the full proposed list — bulk approval).
// Returns the number of rows archived; never deletes anything.
func (s *Service) Apply(ctx context.Context, uris []string, nowMS int64) (int, error) {
	if len(uris) > 0 {
		return s.updateState(ctx, "archived_at = ?", nowMS, uris)
	}
	// Bulk-approve: derive the candidate set in SQL and archive it atomically.
	cutoff := s.now().Add(-time.Duration(ColdWindowDays) * 24 * time.Hour).UnixMilli()
	res, err := s.db.ExecContext(ctx, `
		UPDATE memories SET archived_at = ?
		WHERE superseded_at IS NULL
		  AND archived_at IS NULL
		  AND uri <> ?
		  AND COALESCE((SELECT heat FROM recall_stats WHERE recall_stats.uri = memories.uri), 0) <= ?
		  AND ((SELECT last_use_at FROM recall_stats WHERE recall_stats.uri = memories.uri) IS NULL
		       OR (SELECT last_use_at FROM recall_stats WHERE recall_stats.uri = memories.uri) < ?)
		  AND NOT EXISTS (
		      SELECT 1 FROM corrections c
		      WHERE c.retracted_at IS NULL
		        AND c.target_uris LIKE '%"' || memories.uri || '"%'
		  )`,
		nowMS, profileURI, HeatEpsilon, cutoff)
	if err != nil {
		return 0, fmt.Errorf("archive apply: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// Restore un-archives the given URIs, or every archived row when all is true.
// Returns rows restored.
func (s *Service) Restore(ctx context.Context, uris []string, all bool) (int, error) {
	if all {
		res, err := s.db.ExecContext(ctx, `UPDATE memories SET archived_at = NULL WHERE archived_at IS NOT NULL`)
		if err != nil {
			return 0, fmt.Errorf("restore all: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		return int(n), nil
	}
	if len(uris) == 0 {
		return 0, fmt.Errorf("restore requires at least one uri or --restore-all")
	}
	return s.updateState(ctx, "archived_at = NULL", nil, uris)
}

// updateState runs an idempotent set-archived-state over a set of URIs.
// `setClause` is the column assignment ("archived_at = ?" for applying,
// "archived_at = NULL" for restoring); when it needs a bound value the caller
// passes it via `value`. Chunked under SQLite's variable limit. Only active
// rows are touched.
func (s *Service) updateState(ctx context.Context, setClause string, value any, uris []string) (int, error) {
	const chunk = 400
	total := 0
	for start := 0; start < len(uris); start += chunk {
		end := start + chunk
		if end > len(uris) {
			end = len(uris)
		}
		part := uris[start:end]
		ph := make([]string, len(part))
		args := make([]any, len(part))
		for i, u := range part {
			ph[i] = "?"
			args[i] = u
		}
		q := "UPDATE memories SET " + setClause + " WHERE superseded_at IS NULL AND uri IN (" + strings.Join(ph, ",") + ")"
		var full []any
		if value != nil {
			full = append(full, value)
		}
		full = append(full, args...)
		res, err := s.db.ExecContext(ctx, q, full...)
		if err != nil {
			return total, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += int(n)
	}
	return total, nil
}
