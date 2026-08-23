package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"time"
)

// bodyDateRE finds a leading date inside an event body or abstract ("On
// 2026-07-16 ...", "2026-07-16: ...", "the 2026-07-16 deploy"). Used only as
// the fallback for events whose slug carries no date prefix.
var bodyDateRE = regexp.MustCompile(`(20\d{2}-\d{2}-\d{2})`)

// BackfillOccurredAt completes the occurred_at backfill (issue #30) for
// event memories the slug-date migration could not date: it scans active
// events with NULL occurred_at, extracts the first date mentioned in their
// abstract/body, and stores it (UTC midnight, ms). Rows without any date
// stay NULL and keep falling back to updated_at at read time. Idempotent.
func BackfillOccurredAt(ctx context.Context, database *sql.DB, log *slog.Logger) error {
	rows, err := database.QueryContext(ctx, `
		SELECT id, COALESCE(abstract, '') || ' ' || COALESCE(body, '')
		FROM memories
		WHERE category = 'events' AND occurred_at IS NULL`)
	if err != nil {
		return fmt.Errorf("occurred_at backfill scan: %w", err)
	}
	type fix struct {
		id   string
		when int64
	}
	var fixes []fix
	for rows.Next() {
		var id, text string
		if err := rows.Scan(&id, &text); err != nil {
			rows.Close()
			return err
		}
		if m := bodyDateRE.FindStringSubmatch(text); m != nil {
			t, err := time.ParseInLocation("2006-01-02", m[1], time.UTC)
			if err != nil {
				continue
			}
			fixes = append(fixes, fix{id: id, when: t.UnixMilli()})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, f := range fixes {
		if _, err := database.ExecContext(ctx,
			`UPDATE memories SET occurred_at = ? WHERE id = ?`, f.when, f.id); err != nil {
			return err
		}
	}
	if len(fixes) > 0 && log != nil {
		log.Info("backfilled occurred_at from body dates", "events", len(fixes))
	}
	return nil
}
