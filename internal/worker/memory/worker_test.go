package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
)

// recordingDistiller captures DistillMemory calls and returns a canned body.
type recordingDistiller struct {
	mu      sync.Mutex
	calls   []distillCall
	bodies  map[string]string // category+slug -> body; default otherwise
	callErr error
}

type distillCall struct {
	Category    string
	Slug        string
	AtomsJSON   string
	Corrections []string
	Related     []llm.RelatedEvent
}

func (d *recordingDistiller) DistillMemory(_ context.Context, category, slug, atomsJSON string, corrections []string, related []llm.RelatedEvent) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, distillCall{Category: category, Slug: slug, AtomsJSON: atomsJSON, Corrections: corrections, Related: related})
	if d.callErr != nil {
		return "", d.callErr
	}
	// Body varies per call so version-bump assertions (same-bucket rewrites)
	// don't collapse on identical bodies.
	body := fmt.Sprintf("b-%d", len(d.calls))
	if d.bodies != nil {
		if b, ok := d.bodies[category+"/"+slug]; ok {
			body = b
		}
	}
	return fmt.Sprintf(`{"abstract":"a","body":%q}`, body), nil
}

func (d *recordingDistiller) eventCalls() []distillCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []distillCall
	for _, c := range d.calls {
		if c.Category == model.AtomCategoryEvents {
			out = append(out, c)
		}
	}
	return out
}

func openWorkerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/rmb.db")
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func insertAtom(t *testing.T, database *sql.DB, id, sessionID, category, slug, content string) {
	t.Helper()
	var slugPtr any
	if slug != "" {
		slugPtr = slug
	}
	_, err := database.Exec(`
		INSERT INTO atoms (id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at)
		VALUES (?, ?, ?, 50, NULL, ?, ?, '[]', 1, 1)`,
		id, sessionID, category, slugPtr, content)
	if err != nil {
		t.Fatal(err)
	}
}

func insertPendingSession(t *testing.T, database *sql.DB, id string) {
	t.Helper()
	nowMS := time.Now().UTC().UnixMilli()
	if _, err := database.Exec(`INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, "key-"+id, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, warmup_threshold, updated_at)
		VALUES (?, 'idle', 'idle', 'pending', 2, ?)`, id, nowMS); err != nil {
		t.Fatal(err)
	}
}

func testWorker(database *sql.DB, distiller MemoryDistiller) *Worker {
	cfg, err := config.Default()
	if err != nil {
		panic(err)
	}
	return NewWorker(database, distiller, cfg.Pipeline, nil, nil)
}

func TestDistillBucketSingleAtomEventGoesThroughLLMWithRelated(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	// An earlier problem event that the new event should be able to link.
	problemBody := "get_diff_tag_from_tag_base_editer dict-vs-list bug caused 500 errors on tag solutions"
	insertMemoryRow(t, database, "p1", "rmb://events/2026-07-13-starlink-hs99-vip-500-bug", problemBody)

	// Single-atom event bucket: must still hit the LLM (no fast path) and
	// receive the related problem event as retrieve-then-link input.
	bucket := Bucket{
		Category: model.AtomCategoryEvents,
		Slug:     "2026-07-16-soft-delete-one-tag-solutions",
		URI:      "rmb://events/2026-07-16-soft-delete-one-tag-solutions",
		Atoms:    []model.Atom{{ID: "a1", SessionID: "s1", Category: model.AtomCategoryEvents, Content: "On 2026-07-16 one-tag-diff solutions were soft-deleted after the tag bug."}},
	}

	distiller := &recordingDistiller{}
	w := testWorker(database, distiller)

	pm, err := w.distillBucket(context.Background(), bucket, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pm.Body != "b-1" {
		t.Fatalf("body=%q", pm.Body)
	}
	calls := distiller.eventCalls()
	if len(calls) != 1 {
		t.Fatalf("want exactly one LLM call for single-atom event, got %d", len(calls))
	}
	found := false
	for _, r := range calls[0].Related {
		if r.URI == "rmb://events/2026-07-13-starlink-hs99-vip-500-bug" {
			found = true
		}
	}
	if !found {
		t.Errorf("related problem event not injected: %+v", calls[0].Related)
	}
}

func TestDistillBucketSingleAtomPreferenceKeepsFastPath(t *testing.T) {
	// Covered more compactly in distill_test.go, but re-check that the fast
	// path still bypasses the LLM for non-event categories after the change.
	distiller := &recordingDistiller{callErr: fmt.Errorf("llm must not be called")}
	w := testWorker(nil, distiller)
	bucket := Bucket{
		Category: model.AtomCategoryPreferences,
		Slug:     "atlas",
		URI:      "rmb://preferences/atlas",
		Atoms:    []model.Atom{{Content: "The user uses Atlas for schema comparison."}},
	}
	pm, err := w.distillBucket(context.Background(), bucket, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pm.Body != "The user uses Atlas for schema comparison." {
		t.Fatalf("body=%q", pm.Body)
	}
}

func TestRelatedEventQuery(t *testing.T) {
	scene := "tag-fixes"
	bucket := Bucket{
		Slug: "2026-07-16-soft-delete-one-tag-solutions",
		Atoms: []model.Atom{
			{SceneName: &scene},
		},
	}
	got := relatedEventQuery(bucket)
	for _, want := range []string{"soft", "delete", "tag", "solutions"} {
		if !strings.Contains(got, want) {
			t.Errorf("query %q missing token %q", got, want)
		}
	}
	if strings.Contains(got, "2026") || strings.Contains(got, "one") {
		t.Errorf("query %q should drop date/stopword tokens", got)
	}
	if relatedEventQuery(Bucket{Slug: "2026-07-16"}) != "" {
		t.Error("date-only slug should produce no query")
	}
}

func TestRollupGraduationBar(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	insertPendingSession(t, database, "s1")
	insertPendingSession(t, database, "s2")

	// Single-session preference subject (the call-user-daddy shape): must NOT
	// be promoted to a memory by the first rollup.
	insertAtom(t, database, "p1", "s1", model.AtomCategoryPreferences, "call-user-daddy",
		"The user prefers to be called Daddy.")
	// Single-session event: exempt from the bar (append-only).
	insertAtom(t, database, "e1", "s1", model.AtomCategoryEvents, "2026-07-16-fix-tag-bug",
		"On 2026-07-16 the tag bug was fixed.")

	w := testWorker(database, &recordingDistiller{})
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}

	var prefCount, eventCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM memories WHERE uri = 'rmb://preferences/call-user-daddy'`).Scan(&prefCount); err != nil {
		t.Fatal(err)
	}
	if prefCount != 0 {
		t.Fatalf("single-session preference must stay un-promoted (graduation bar), got %d rows", prefCount)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM memories WHERE uri = 'rmb://events/2026-07-16-fix-tag-bug'`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("single-session event must persist (exempt from bar), got %d rows", eventCount)
	}

	// A second, distinct session corroborating the subject graduates it.
	insertPendingSession(t, database, "s3")
	insertAtom(t, database, "p2", "s3", model.AtomCategoryPreferences, "call-user-daddy",
		"The user prefers to be called Daddy.")
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM memories WHERE uri = 'rmb://preferences/call-user-daddy' AND superseded_at IS NULL`).Scan(&prefCount); err != nil {
		t.Fatal(err)
	}
	if prefCount != 1 {
		t.Fatalf("two-session subject should graduate, got %d rows", prefCount)
	}

	// Already-graduated subjects rewrite as before: a NEW source scene (not a
	// session bar) triggers the re-distill and version bump.
	insertAtom(t, database, "p3", "s4", model.AtomCategoryPreferences, "call-user-daddy",
		"The user prefers to be called Daddy in AI chats.")
	insertScene(t, database, "sc1", "s4", []string{"p3"})
	insertPendingSession(t, database, "s4")
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := database.QueryRow(`SELECT version FROM memories WHERE uri = 'rmb://preferences/call-user-daddy' AND superseded_at IS NULL`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version < 2 {
		t.Fatalf("graduated subject should version on new evidence, version=%d", version)
	}
}

func TestGraduationDeferredExistingMemoryRewrites(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	insertMemoryRow(t, database, "m1", "rmb://entities/starlink", "starlink RDS entity")
	w := testWorker(database, &recordingDistiller{})

	slug := "starlink"
	bucket := Bucket{
		Category: model.AtomCategoryEntities,
		Slug:     slug,
		URI:      "rmb://entities/starlink",
		Atoms:    []model.Atom{{ID: "a1", SessionID: "only-session", Category: model.AtomCategoryEntities, Content: "starlink entity fact"}},
	}
	deferred, err := w.graduationDeferred(context.Background(), bucket)
	if err != nil {
		t.Fatal(err)
	}
	if deferred {
		t.Error("existing graduated subject must not be deferred by the session bar")
	}
}

func insertScene(t *testing.T, database *sql.DB, id, sessionID string, atomIDs []string) {
	t.Helper()
	sourceJSON, err := db.MarshalStringArray(atomIDs)
	if err != nil {
		t.Fatal(err)
	}
	nowMS := time.Now().UTC().UnixMilli()
	if _, err := database.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, created_at, updated_at)
		VALUES (?, ?, 'scene', 'scene', 'scene body', ?, ?, ?)`, id, sessionID, sourceJSON, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
}

func insertMemoryRow(t *testing.T, database *sql.DB, id, uri, body string) {
	t.Helper()
	category := "events"
	if strings.Contains(uri, "/entities/") {
		category = "entities"
	} else if strings.Contains(uri, "/preferences/") {
		category = "preferences"
	}
	nowMS := time.Now().UTC().UnixMilli()
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, '[]', '[]', ?, ?)`, id, uri, category, body, body, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`, id, body, body); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunL3(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	insertPendingSession(t, database, "s1")
	insertMemoryRow(t, database, "p1", "rmb://events/2026-07-13-starlink-hs99-vip-500-bug",
		"get_diff_tag_from_tag_base_editer dict-vs-list bug on tag solutions")
	insertAtom(t, database, "a1", "s1", model.AtomCategoryEvents, "2026-07-16-soft-delete-one-tag-solutions",
		"On 2026-07-16 one-tag-diff solutions were soft-deleted; verified 67 rows remain.")
	insertAtom(t, database, "a2", "s1", model.AtomCategoryPreferences, "solo-subject",
		"The user prefers to be called Daddy.")

	cfg, err := config.Default()
	if err != nil {
		t.Fatal(err)
	}
	distiller := &recordingDistiller{}
	result, err := DryRunL3(context.Background(), database, distiller, cfg.Pipeline, nil, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionID != "s1" || len(result.Buckets) != 2 {
		t.Fatalf("session=%s buckets=%d", result.SessionID, len(result.Buckets))
	}
	for _, b := range result.Buckets {
		switch b.URI {
		case "rmb://events/2026-07-16-soft-delete-one-tag-solutions":
			if b.Decision != "distilled" {
				t.Errorf("event bucket decision=%s", b.Decision)
			}
			if b.RelatedCount == 0 {
				t.Error("event bucket should report related-event candidates")
			}
			if b.Body != "b-1" {
				t.Errorf("dry-run body=%q", b.Body)
			}
		case "rmb://preferences/solo-subject":
			if b.Decision != "graduation-deferred" {
				t.Errorf("single-session preference decision=%s", b.Decision)
			}
			if b.Body != "" {
				t.Error("deferred bucket should not produce a body")
			}
		}
	}
	// Nothing persisted.
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM memories WHERE uri LIKE '%soft-delete-one-tag%' OR uri LIKE '%solo-subject%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("dry-run must not persist memories, got %d", n)
	}
}
