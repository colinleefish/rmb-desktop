package correction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/uri"
	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid correction input")
	ErrNotFound     = errors.New("correction not found")
)

type Summary struct {
	URI       string `json:"uri"`
	Statement string `json:"statement"`
	CreatedAt string `json:"created_at"`
}

type Row struct {
	ID         string
	TargetURIs []string
	Statement  string
	CreatedAt  time.Time
}

type CreateInput struct {
	TargetURIs []string
	Statement  string
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(database *sql.DB) *Service {
	return &Service{db: database, now: time.Now}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Summary, error) {
	targets, err := normalizeTargets(in.TargetURIs)
	if err != nil {
		return Summary{}, err
	}
	if len(targets) == 0 {
		return Summary{}, fmt.Errorf("%w: a correction requires at least one target URI", ErrInvalidInput)
	}

	statement := strings.TrimSpace(in.Statement)
	if statement == "" {
		return Summary{}, fmt.Errorf("%w: a correction requires a statement", ErrInvalidInput)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Summary{}, fmt.Errorf("generate correction id: %w", err)
	}
	targetJSON, err := db.MarshalStringArray(targets)
	if err != nil {
		return Summary{}, err
	}

	now := s.now().UTC()
	nowMS := now.UnixMilli()
	correctionURI := uri.BuildCorrection(id.String())

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO corrections (id, target_uris, statement, created_at)
		VALUES (?, ?, ?, ?)`,
		id.String(), targetJSON, statement, nowMS,
	)
	if err != nil {
		return Summary{}, fmt.Errorf("insert correction: %w", err)
	}

	return Summary{
		URI:       correctionURI,
		Statement: statement,
		CreatedAt: now.Format(time.RFC3339),
	}, nil
}

func (s *Service) Retract(ctx context.Context, correctionURI string) ([]string, error) {
	u, err := uri.Parse(strings.TrimSpace(correctionURI))
	if err != nil || u.Scope != uri.ScopeCorrections || len(u.Segments) != 1 {
		return nil, fmt.Errorf("%w: not a correction URI: %q", ErrInvalidInput, correctionURI)
	}
	id := strings.ToLower(u.Segments[0])

	var targetJSON string
	err = s.db.QueryRowContext(ctx, `
		SELECT target_uris FROM corrections
		WHERE id = ? AND retracted_at IS NULL`, id,
	).Scan(&targetJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, correctionURI)
	}
	if err != nil {
		return nil, fmt.Errorf("load correction: %w", err)
	}

	nowMS := s.now().UTC().UnixMilli()
	res, err := s.db.ExecContext(ctx, `
		UPDATE corrections SET retracted_at = ?
		WHERE id = ? AND retracted_at IS NULL`, nowMS, id,
	)
	if err != nil {
		return nil, fmt.Errorf("retract correction: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, correctionURI)
	}

	targets, err := db.UnmarshalStringArray(targetJSON)
	if err != nil {
		return nil, err
	}
	return targets, nil
}

func (s *Service) List(ctx context.Context, target string, limit, offset int) ([]Summary, int64, error) {
	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	target = strings.TrimSpace(target)
	var total int64
	var rows *sql.Rows
	var err error

	if target != "" {
		if _, err := uri.Parse(target); err != nil {
			return nil, 0, fmt.Errorf("%w: target %q: %v", ErrInvalidInput, target, err)
		}
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM corrections
			WHERE retracted_at IS NULL
			AND EXISTS (SELECT 1 FROM json_each(target_uris) WHERE value = ?)`,
			target,
		).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count corrections: %w", err)
		}
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, statement, created_at FROM corrections
			WHERE retracted_at IS NULL
			AND EXISTS (SELECT 1 FROM json_each(target_uris) WHERE value = ?)
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?`, target, limit, offset)
	} else {
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM corrections WHERE retracted_at IS NULL`,
		).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count corrections: %w", err)
		}
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, statement, created_at FROM corrections
			WHERE retracted_at IS NULL
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list corrections: %w", err)
	}
	defer rows.Close()

	var items []Summary
	for rows.Next() {
		var id, statement string
		var createdMS int64
		if err := rows.Scan(&id, &statement, &createdMS); err != nil {
			return nil, 0, err
		}
		items = append(items, Summary{
			URI:       uri.BuildCorrection(id),
			Statement: statement,
			CreatedAt: time.UnixMilli(createdMS).UTC().Format(time.RFC3339),
		})
	}
	return items, total, rows.Err()
}

func ForTargets(ctx context.Context, database *sql.DB, targetURIs []string) (map[string][]Summary, error) {
	wanted := make(map[string]struct{}, len(targetURIs))
	for _, u := range targetURIs {
		if u != "" {
			wanted[u] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return map[string][]Summary{}, nil
	}

	rows, err := database.QueryContext(ctx, `
		SELECT id, target_uris, statement, created_at FROM corrections
		WHERE retracted_at IS NULL
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("load corrections: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]Summary)
	for rows.Next() {
		var id, targetJSON, statement string
		var createdMS int64
		if err := rows.Scan(&id, &targetJSON, &statement, &createdMS); err != nil {
			return nil, err
		}
		targets, err := db.UnmarshalStringArray(targetJSON)
		if err != nil {
			return nil, err
		}
		sum := Summary{
			URI:       uri.BuildCorrection(id),
			Statement: statement,
			CreatedAt: time.UnixMilli(createdMS).UTC().Format(time.RFC3339),
		}
		for _, t := range targets {
			if _, ok := wanted[t]; ok {
				out[t] = append(out[t], sum)
			}
		}
	}
	return out, rows.Err()
}

func normalizeTargets(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		t := strings.TrimSpace(r)
		if t == "" {
			continue
		}
		u, err := uri.Parse(t)
		if err != nil {
			return nil, fmt.Errorf("%w: target %q: %v", ErrInvalidInput, r, err)
		}
		canonical := u.String()
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	return out, nil
}

func SplitSummaries(sums []Summary) (statements, uris []string) {
	for _, s := range sums {
		uris = append(uris, s.URI)
		if strings.TrimSpace(s.Statement) != "" {
			statements = append(statements, s.Statement)
		}
	}
	return statements, uris
}
