package recall_test

import (
	"context"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/recall"
)

func TestParseTimeValue_relative(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		raw  string
		want int64 // ms delta from now (negative = past)
	}{
		{"30m", -30 * 60 * 1000},
		{"12h", -12 * 60 * 60 * 1000},
		{"7d", -7 * 24 * 60 * 60 * 1000},
	}
	for _, c := range cases {
		got, err := recall.ParseTimeValue(c.raw, now)
		if err != nil {
			t.Fatalf("%s: %v", c.raw, err)
		}
		if got != now.UnixMilli()+c.want {
			t.Errorf("%s: got delta %d want %d", c.raw, got-now.UnixMilli(), c.want)
		}
	}
}

func TestParseTimeValue_absolute(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	got, err := recall.ParseTimeValue("2026-08-01", now)
	if err != nil {
		t.Fatal(err)
	}
	// date-only resolves to local midnight; compare the calendar date in UTC.
	back := time.UnixMilli(got).UTC()
	if back.Format("2006-01-02") != "2026-08-01" || back.Hour() != 0 || back.Minute() != 0 {
		t.Errorf("date-only parse: %v", back)
	}

	got, err = recall.ParseTimeValue("2026-08-01T15:04", now)
	if err != nil {
		t.Fatal(err)
	}
	back = time.UnixMilli(got).UTC()
	if back.Format("2006-01-02T15:04") != "2026-08-01T15:04" {
		t.Errorf("datetime parse: %v", back)
	}
}

func TestParseTimeValue_rejectsGarbage(t *testing.T) {
	now := time.Now()
	for _, bad := range []string{"", "yesterday", "2026-13-99", "7x", "d"} {
		if _, err := recall.ParseTimeValue(bad, now); err == nil {
			t.Errorf("%q should error", bad)
		}
	}
}

func TestTimeWindow_Clause(t *testing.T) {
	clause, args := recall.TimeWindow{}.Clause("updated_at")
	if clause != "" || args != nil {
		t.Error("zero window must be a no-op")
	}
	clause, args = recall.TimeWindow{SinceMS: 100, UntilMS: 200}.Clause("updated_at")
	want := " AND updated_at >= ? AND updated_at <= ?"
	if clause != want || len(args) != 2 || args[0].(int64) != 100 || args[1].(int64) != 200 {
		t.Errorf("clause=%q args=%v", clause, args)
	}
}

func TestSearch_FTS_memories_timeWindow(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	oldMS := nowMS - 40*24*time.Hour.Milliseconds() // ~40 days ago

	insert := func(id, uri string, ts int64) {
		t.Helper()
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
			VALUES (?, ?, 'entities', 1, 'k8s', 'kubectl apply deployment yaml', '[]', '[]', ?, ?)`,
			id, uri, ts, ts); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories_fts(rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), 'k8s', 'kubectl apply deployment yaml')`, id); err != nil {
			t.Fatal(err)
		}
	}
	insert("22222222-2222-4222-8222-222222222222", "rmb://entities/new", nowMS)
	insert("33333333-3333-4333-8333-333333333333", "rmb://entities/old", oldMS)

	svc := recall.NewService(database)

	// Unfiltered: both hit.
	m, err := svc.Search(context.Background(), nil, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("unfiltered: want 2 hits, got %d", len(m))
	}

	// --since=7d: only the fresh row.
	m, err = svc.Search(context.Background(), nil, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{SinceMS: nowMS - 7*24*time.Hour.Milliseconds()})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0].URI != "rmb://entities/new" {
		t.Fatalf("since=7d: got %v", m)
	}

	// --until in the past: only the stale row.
	m, err = svc.Search(context.Background(), nil, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{UntilMS: oldMS + 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0].URI != "rmb://entities/old" {
		t.Fatalf("until: got %v", m)
	}

	// Boundary equality: since exactly at old timestamp includes the old row.
	m, err = svc.Search(context.Background(), nil, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{SinceMS: oldMS})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("since=oldMS boundary: want 2, got %d", len(m))
	}
}
