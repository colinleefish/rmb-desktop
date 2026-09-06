package hygiene

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/model"
)

func openHygieneDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func mustExec(t *testing.T, d *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := d.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func insertVersionRow(t *testing.T, d *sql.DB, id, uri string, version int, superseded bool, updatedAt int64) {
	t.Helper()
	var sup any
	if superseded {
		sup = updatedAt
	}
	mustExec(t, d, `
		INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES (?, ?, 'entities', ?, ?, 'a', ?, '[]', ?, ?)`,
		id, uri, version, sup, "body text for "+id, updatedAt, updatedAt)
}

func daysAgo(n int) int64 { return time.Now().Add(-time.Duration(n) * 24 * time.Hour).UnixMilli() }

// TestSupersededGC_KeepThreeOr90Days (issue #33): per URI keep the newer of
// (3 newest superseded versions, versions within 90d); actives never touched.
func TestSupersededGC_KeepThreeOr90Days(t *testing.T) {
	d := openHygieneDB(t)
	defer d.Close()

	uri := "rmb://entities/aliyun"
	insertVersionRow(t, d, "v0", uri, 6, false, daysAgo(1)) // active — untouchable
	insertVersionRow(t, d, "v1", uri, 5, true, daysAgo(2))
	insertVersionRow(t, d, "v2", uri, 4, true, daysAgo(3))
	insertVersionRow(t, d, "v3", uri, 3, true, daysAgo(4))
	insertVersionRow(t, d, "v4", uri, 2, true, daysAgo(40))  // within 90d -> kept
	insertVersionRow(t, d, "v5", uri, 1, true, daysAgo(120)) // older than 90d -> reclaimable

	stats, err := SupersededGC(t.Context(), d, 3, 90, true)
	if err != nil {
		t.Fatal(err)
	}
	if !stats.DryRun {
		t.Fatal("expected dry run")
	}
	if stats.DeletedRows != 1 {
		t.Fatalf("dry-run: want 1 reclaimable (the 120d-old row), got %d", stats.DeletedRows)
	}

	if _, err := SupersededGC(t.Context(), d, 3, 90, false); err != nil {
		t.Fatal(err)
	}
	var active, superseded, gone int
	mustScan(t, d, `SELECT COUNT(*) FROM memories WHERE uri=? AND superseded_at IS NULL`, &active, uri)
	mustScan(t, d, `SELECT COUNT(*) FROM memories WHERE uri=? AND superseded_at IS NOT NULL`, &superseded, uri)
	mustScan(t, d, `SELECT COUNT(*) FROM memories WHERE id='v5'`, &gone)
	if active != 1 {
		t.Fatalf("active row must survive, got %d", active)
	}
	if superseded != 4 {
		t.Fatalf("keep max(3 newest, 90d) = 4 rows here, got %d", superseded)
	}
	if gone != 0 {
		t.Fatal("the 120d-old superseded row must be deleted on apply")
	}
}

func mustScan(t *testing.T, d *sql.DB, query string, dest any, args ...any) {
	t.Helper()
	if err := d.QueryRow(query, args...).Scan(dest); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// TestReport_Scans (issue #33 acceptance): the report surfaces every health
// dimension on demand.
func TestReport_Scans(t *testing.T) {
	d := openHygieneDB(t)
	defer d.Close()

	now := time.Now().UTC().UnixMilli()
	// Date-less event slug.
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES ('e1', 'rmb://events/some-undated-event', 'events', 1, NULL, 'a', 'b', '[]', ?, ?)`, now, now)
	// Over-cap body + a v131 chain (the audit's erosion case).
	long := strings.Repeat("x", model.MaxBodyChars+100)
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES ('h1', 'rmb://entities/eroded', 'entities', 131, NULL, 'a', ?, '[]', ?, ?)`, long, now, now)
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES ('s4', 'rmb://entities/eroded-old', 'entities', 127, ?, 'a', 'b', '[]', ?, ?)`, now, daysAgo(4), daysAgo(4))
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES ('s3', 'rmb://entities/eroded-old', 'entities', 128, ?, 'a', 'b', '[]', ?, ?)`, now, daysAgo(3), daysAgo(3))
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES ('s2', 'rmb://entities/eroded-old', 'entities', 129, ?, 'a', 'b', '[]', ?, ?)`, now, daysAgo(2), daysAgo(2))
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES ('s1', 'rmb://entities/eroded-old', 'entities', 130, ?, 'a', 'b', '[]', ?, ?)`, now, daysAgo(200), daysAgo(200))
	// An orphan scene referenced by no active memory.
	mustExec(t, d, `INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES ('sess1', 'k1', ?, ?)`, now, now)
	mustExec(t, d, `INSERT INTO scenes (id, session_id, abstract, body, source_atoms, created_at, updated_at)
		VALUES ('orphan-scene-1', 'sess1', 'a', 'b', '[]', ?, ?)`, now, now)

	rep, err := Report(t.Context(), d, 50)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DatelessEventSlugs != 1 {
		t.Errorf("dateless event slugs: got %d want 1", rep.DatelessEventSlugs)
	}
	if rep.BodiesOverCap != 1 {
		t.Errorf("bodies over cap: got %d want 1", rep.BodiesOverCap)
	}
	if rep.HighVersionCount != 1 {
		t.Errorf("high-version chains: got %d want 1", rep.HighVersionCount)
	}
	if rep.OrphanScenes != 1 {
		t.Errorf("orphan scenes: got %d want 1", rep.OrphanScenes)
	}
	if rep.GCReclaimableRows != 1 {
		t.Errorf("gc reclaimable: got %d want 1 (the 200d-old superseded row)", rep.GCReclaimableRows)
	}
}

// TestDuplicatePairs_NearIdenticalEmbeddings: same-category actives with
// near-identical embeddings surface as duplicate candidates; a contradicting
// pair (one side negated) is flagged for review.
func TestDuplicatePairs_NearIdenticalEmbeddings(t *testing.T) {
	d := openHygieneDB(t)
	defer d.Close()

	now := time.Now().UTC().UnixMilli()
	// Identical embedding blobs (2-dim) for two entities; one negated body.
	vec := []byte{0, 0, 128, 63, 0, 0, 128, 63} // {1.0, 1.0} LE float32
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, embedding, created_at, updated_at)
		VALUES ('d1', 'rmb://entities/alpha', 'entities', 1, NULL, 'a', 'Redis auth is always required.', '[]', ?, ?, ?)`, vec, now, now)
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, embedding, created_at, updated_at)
		VALUES ('d2', 'rmb://entities/alpha2', 'entities', 1, NULL, 'a', 'Redis auth is never required.', '[]', ?, ?, ?)`, vec, now, now)
	// Distinct embedding elsewhere.
	vec2 := []byte{0, 0, 128, 63, 0, 0, 0, 0}
	mustExec(t, d, `INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, embedding, created_at, updated_at)
		VALUES ('d3', 'rmb://entities/beta', 'entities', 1, NULL, 'a', 'Totally unrelated system.', '[]', ?, ?, ?)`, vec2, now, now)

	rep, err := Report(t.Context(), d, 50)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DuplicatePairs != 1 {
		t.Fatalf("duplicate pairs: got %d want 1", rep.DuplicatePairs)
	}
	if rep.TopDuplicatePairs[0].A != "rmb://entities/alpha" || rep.TopDuplicatePairs[0].B != "rmb://entities/alpha2" {
		t.Errorf("wrong pair: %+v", rep.TopDuplicatePairs[0])
	}
	if len(rep.ContradictionCandidates) != 1 {
		t.Fatalf("contradiction candidates: got %d want 1 (always vs never)", len(rep.ContradictionCandidates))
	}
	if rep.ContradictionCandidates[0].NegatedSide != "rmb://entities/alpha2" {
		t.Errorf("negated side should be the 'never' body, got %s", rep.ContradictionCandidates[0].NegatedSide)
	}
}
