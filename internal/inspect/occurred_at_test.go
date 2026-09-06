package inspect

import (
	"strings"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
)

// insertEvent inserts an active event memory with explicit timestamps and an
// optional occurred_at.
func insertEvent(t *testing.T, s *Service, uri string, createdAt, updatedAt int64, occurredAt *int64) {
	t.Helper()
	var occurred any
	if occurredAt != nil {
		occurred = *occurredAt
	}
	if _, err := s.db.Exec(`
		INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, occurred_at, created_at, updated_at)
		VALUES (?, ?, 'events', 1, NULL, '', '', '[]', ?, ?, ?)`,
		"mem-"+uri, uri, occurred, createdAt, updatedAt); err != nil {
		t.Fatalf("insert event %s: %v", uri, err)
	}
}

func ms(t time.Time) int64 { return t.UnixMilli() }

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// TestEventsLsOrdersByOccurredAt (issue #30 acceptance): the 08-13 backfill
// batch (June/July slugs, August write-time) must sort under June/July, not
// August. --by=updated restores the write-time view.
func TestEventsLsOrdersByOccurredAt(t *testing.T) {
	s := newTestService(t)

	aug := ms(d(2026, 8, 13)) // the backfill batch's WRITE time
	june := ms(d(2026, 6, 13))
	july := ms(d(2026, 7, 29))
	insertEvent(t, s, "rmb://events/2026-06-13-ding-lby-pilot", aug, aug, &june)
	insertEvent(t, s, "rmb://events/2026-07-29-kechuan-second-wave", aug, aug, &july)
	// A genuinely-August event (written and occurred in August).
	insertEvent(t, s, "rmb://events/2026-08-21-bbc-param-split", ms(d(2026, 8, 21)), ms(d(2026, 8, 21)), nil)

	got := lsLines(t, s, "rmb://events/", DefaultLsOptions())
	want := []string{
		"rmb://events/2026-08-21-bbc-param-split", // August happened last
		"rmb://events/2026-07-29-kechuan-second-wave",
		"rmb://events/2026-06-13-ding-lby-pilot", // backfilled June sorts under June
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events must order by occurred_at: got %v", got)
	}

	// Opt-out: write-time view puts the backfill batch (updated 08-13) on top.
	byUpdated := DefaultLsOptions()
	byUpdated.ByUpdated = true
	gotU := lsLines(t, s, "rmb://events/", byUpdated)
	if gotU[0] != "rmb://events/2026-08-21-bbc-param-split" || gotU[1] != "rmb://events/2026-06-13-ding-lby-pilot" {
		t.Fatalf("--by=updated must order by updated_at: got %v", gotU)
	}
}

// TestEventsSinceFiltersByOccurredAt: --since/--until on events filter by
// when things happened, not write time (issue #30).
func TestEventsSinceFiltersByOccurredAt(t *testing.T) {
	s := newTestService(t)
	aug := ms(d(2026, 8, 13))
	june := ms(d(2026, 6, 13))
	insertEvent(t, s, "rmb://events/2026-06-13-ding-lby-pilot", aug, aug, &june)
	insertEvent(t, s, "rmb://events/2026-08-21-bbc-param-split", ms(d(2026, 8, 21)), ms(d(2026, 8, 21)), nil)

	opts := DefaultLsOptions()
	opts.Since = ms(d(2026, 7, 1))
	opts.Until = ms(d(2026, 8, 31))
	got := lsLines(t, s, "rmb://events/", opts)
	if len(got) != 1 || got[0] != "rmb://events/2026-08-21-bbc-param-split" {
		t.Fatalf("--since window must filter by occurred_at (June event excluded), got %v", got)
	}
}

// TestEventsOccurredAtNullFallsBackToUpdated: undatable events keep the
// write-time view (COALESCE fallback), so nothing disappears.
func TestEventsOccurredAtNullFallsBackToUpdated(t *testing.T) {
	s := newTestService(t)
	insertEvent(t, s, "rmb://events/duckdb-to-postgres-pbp", ms(d(2026, 7, 11)), ms(d(2026, 7, 11)), nil)
	insertEvent(t, s, "rmb://events/2026-08-21-later", ms(d(2026, 8, 21)), ms(d(2026, 8, 21)), nil)
	got := lsLines(t, s, "rmb://events/", DefaultLsOptions())
	if len(got) != 2 || got[0] != "rmb://events/2026-08-21-later" {
		t.Fatalf("NULL occurred_at must fall back to updated_at, got %v", got)
	}
}

// TestBackfillOccurredAtFromBodies: the Go fallback dates events the slug
// migration could not (dates mentioned in abstract/body).
func TestBackfillOccurredAtFromBodies(t *testing.T) {
	s := newTestService(t)
	if _, err := s.db.Exec(`
		INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES ('e1', 'rmb://events/duckdb-to-postgres-pbp', 'events', 1, NULL,
			'PBP migrated from DuckDB to Postgres.',
			'On 2026-07-11 PBP migrated from DuckDB/OSS Parquet to Postgres (pbp_db).',
			'[]', ?, ?)`, ms(d(2026, 8, 13)), ms(d(2026, 8, 13))); err != nil {
		t.Fatal(err)
	}

	if err := db.BackfillOccurredAt(t.Context(), s.db, nil); err != nil {
		t.Fatal(err)
	}
	var occurred int64
	if err := s.db.QueryRow(`SELECT occurred_at FROM memories WHERE id = 'e1'`).Scan(&occurred); err != nil {
		t.Fatal(err)
	}
	if want := ms(d(2026, 7, 11)); occurred != want {
		t.Fatalf("body date backfill: got %d want %d", occurred, want)
	}

	// Idempotent: a second run changes nothing.
	if err := db.BackfillOccurredAt(t.Context(), s.db, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT occurred_at FROM memories WHERE id = 'e1'`).Scan(&occurred); err != nil {
		t.Fatal(err)
	}
	if occurred != ms(d(2026, 7, 11)) {
		t.Fatal("backfill must be idempotent")
	}
}
