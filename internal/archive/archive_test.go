package archive_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/archive"
	"github.com/colinleefish/rmb-desktop/internal/db"
)

const dayMS = int64(24 * 3600 * 1000)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

type memOpts struct {
	superseded *int64
	archived   *int64
	heat       *float64
	lastUseAt  *int64
	statsRow   bool
	correction bool
	updatedMS  int64
}

// insertMem inserts an active memory (with optional FTS row) and, per opts,
// a recall_stats row and/or a correction targeting it.
func insertMem(t *testing.T, database *sql.DB, id, uri, abs string, o memOpts) {
	t.Helper()
	if o.updatedMS == 0 {
		o.updatedMS = time.Now().UTC().UnixMilli()
	}
	sup := o.superseded
	arch := o.archived
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, superseded_at, archived_at, created_at, updated_at)
		VALUES (?, ?, 'entities', 1, ?, '', '[]', '[]', ?, ?, ?, ?)`,
		id, uri, abs, sup, arch, o.updatedMS, o.updatedMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, '')`, id, abs); err != nil {
		t.Fatal(err)
	}
	if o.statsRow {
		h := 0.0
		if o.heat != nil {
			h = *o.heat
		}
		if _, err := database.Exec(`
			INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, heat, last_use_at, updated_at)
			VALUES (?, 0, 0, 0, ?, ?, ?)`,
			uri, h, o.lastUseAt, o.updatedMS); err != nil {
			t.Fatal(err)
		}
	}
	if o.correction {
		if _, err := database.Exec(`
			INSERT INTO corrections (id, target_uris, statement, created_at, retracted_at)
			VALUES (?, ?, 'correction text', ?, NULL)`,
			id+"-corr", `["`+uri+`"]`, o.updatedMS); err != nil {
			t.Fatal(err)
		}
	}
}

func uris(cands []archive.Candidate) []string {
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.URI)
	}
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func TestCandidates_policy(t *testing.T) {
	database := openDB(t)
	now := time.Now().UTC()
	nowMS := now.UnixMilli()
	old := nowMS - 200*dayMS

	svc := archive.NewService(database)
	svc.SetClock(func() time.Time { return now })

	insertMem(t, database, "p-1", "rmb://profile", "who the user is", memOpts{updatedMS: old})
	insertMem(t, database, "a-1", "rmb://entities/cold-a", "cold fact a", memOpts{updatedMS: old})
	insertMem(t, database, "b-1", "rmb://events/cold-b", "cold event b", memOpts{updatedMS: old})
	insertMem(t, database, "c-1", "rmb://preferences/recently-used", "used yesterday",
		memOpts{updatedMS: old, statsRow: true, lastUseAt: &[]int64{nowMS - dayMS}[0]})
	insertMem(t, database, "d-1", "rmb://entities/warm-heat", "warm", memOpts{updatedMS: old, statsRow: true, heat: &[]float64{5}[0]})
	insertMem(t, database, "e-1", "rmb://entities/correction-linked", "linked",
		memOpts{updatedMS: old, correction: true})
	sup := nowMS - 10*dayMS
	insertMem(t, database, "f-1", "rmb://events/superseded", "old version", memOpts{updatedMS: old, superseded: &sup})
	arch := nowMS - 5*dayMS
	insertMem(t, database, "g-1", "rmb://entities/already-archived", "gone", memOpts{updatedMS: old, archived: &arch})

	got := uris(mustCandidates(t, svc, 90))
	for _, want := range []string{"rmb://entities/cold-a", "rmb://events/cold-b"} {
		if !contains(got, want) {
			t.Errorf("candidate %s missing; got %v", want, got)
		}
	}
	for _, banned := range []string{
		"rmb://profile",
		"rmb://preferences/recently-used",
		"rmb://entities/warm-heat",
		"rmb://entities/correction-linked",
		"rmb://events/superseded",
		"rmb://entities/already-archived",
	} {
		if contains(got, banned) {
			t.Errorf("excluded memory %s wrongly proposed: %v", banned, got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 candidates, got %v", got)
	}
}

func TestApplyRestore_roundTrip(t *testing.T) {
	database := openDB(t)
	now := time.Now().UTC()
	nowMS := now.UnixMilli()
	old := nowMS - 200*dayMS

	svc := archive.NewService(database)
	svc.SetClock(func() time.Time { return now })

	insertMem(t, database, "a-1", "rmb://entities/cold-a", "cold fact a", memOpts{updatedMS: old})
	insertMem(t, database, "b-1", "rmb://events/cold-b", "cold event b", memOpts{updatedMS: old})

	// Bulk apply the proposed set (empty uris).
	n, err := svc.Apply(context.Background(), nil, nowMS)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("bulk apply archived %d, want 2", n)
	}
	if got := uris(mustCandidates(t, svc, 90)); len(got) != 0 {
		t.Fatalf("after apply, candidates should be empty, got %v", got)
	}

	// Restore a single uri.
	n, err = svc.Restore(context.Background(), []string{"rmb://entities/cold-a"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("restore single archived %d, want 1", n)
	}
	got := uris(mustCandidates(t, svc, 90))
	if !contains(got, "rmb://entities/cold-a") {
		t.Fatalf("restored cold-a should be a candidate again: %v", got)
	}
	if contains(got, "rmb://events/cold-b") {
		t.Fatalf("cold-b should still be archived: %v", got)
	}

	// Restore everything.
	if n, err = svc.Restore(context.Background(), nil, true); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("restore-all restored %d (only cold-b remained), want 1", n)
	}
	if got := uris(mustCandidates(t, svc, 90)); len(got) != 2 {
		t.Fatalf("after restore-all, both should be candidates again: %v", got)
	}
}

func mustCandidates(t *testing.T, svc *archive.Service, days int) []archive.Candidate {
	t.Helper()
	c, err := svc.Candidates(context.Background(), days)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
