package extract

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
)

func openExtractTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/rmb.db")
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func insertExistingAtom(t *testing.T, database *sql.DB, id, category, slug, content string) {
	t.Helper()
	var slugPtr any
	if slug != "" {
		slugPtr = slug
	}
	if _, err := database.Exec(`
		INSERT INTO atoms (id, session_id, category, priority, scene_name, slug, content, source_turn_ids, created_at, updated_at)
		VALUES (?, 'seed-session', ?, 50, NULL, ?, ?, '[]', 1, 1)`,
		id, category, slugPtr, content); err != nil {
		t.Fatal(err)
	}
}

func insertExistingMemory(t *testing.T, database *sql.DB, id, category, slug, body string) {
	t.Helper()
	uri := "rmb://" + category + "/" + slug
	nowMS := time.Now().UTC().UnixMilli()
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, slug, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?, '[]', '[]', ?, ?)`, id, uri, category, slug, body, body, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`, id, body, body); err != nil {
		t.Fatal(err)
	}
}

// TestCanonicalizeAtoms_EventDatePrefix (issue #27 task 2): event slugs
// without a YYYY-MM-DD- prefix get the session date prepended at extract
// time; already-dated slugs and other categories pass through untouched.
func TestCanonicalizeAtoms_EventDatePrefix(t *testing.T) {
	turns := []model.SessionTurn{{CreatedAt: time.Date(2026, 8, 22, 10, 0, 0, 0, time.Local).UnixMilli()}}
	parsed := []llmAtom{
		{Category: model.AtomCategoryEvents, Slug: "fix-tag-bug", Content: "On 2026-08-22 the tag bug was fixed."},
		{Category: model.AtomCategoryEvents, Slug: "2026-08-21-bbc-deploy-split", Content: "dated already"},
		{Category: model.AtomCategoryEntities, Slug: "jenkins", Content: "Jenkins home is /var/lib/jenkins."},
	}
	got := canonicalizeAtoms(parsed, nil, turns)
	if got[0].Slug != "2026-08-22-fix-tag-bug" {
		t.Errorf("undated event slug must gain the session date prefix, got %q", got[0].Slug)
	}
	if got[1].Slug != "2026-08-21-bbc-deploy-split" {
		t.Errorf("dated slug must stay untouched, got %q", got[1].Slug)
	}
	if got[2].Slug != "jenkins" {
		t.Errorf("entity slug must stay untouched, got %q", got[2].Slug)
	}
}

// TestCanonicalizeAtoms_SlugCandidateReuse (issue #27 task 2): a variant
// spelling that normalizes equal to a retrieved candidate ("doc-language" vs
// "docs-language") is rewritten to the incumbent slug, so L3 buckets
// consolidate. Unrelated slugs pass through.
func TestCanonicalizeAtoms_SlugCandidateReuse(t *testing.T) {
	cands := []llm.SlugCandidate{
		{Category: "preferences", Slug: "docs-language"},
		{Category: "entities", Slug: "jenkins"},
		{Category: "preferences", Slug: "redis-credentials"},
	}
	parsed := []llmAtom{
		{Category: model.AtomCategoryPreferences, Slug: "doc-language", Content: "Docs in Chinese."},
		{Category: model.AtomCategoryEntities, Slug: "jira", Content: "Jira is used for tickets."},
		{Category: model.AtomCategoryPreferences, Slug: "redis-credential", Content: "variant"},
	}
	got := canonicalizeAtoms(parsed, cands, nil)
	if got[0].Slug != "docs-language" {
		t.Errorf("variant must canonicalize to the incumbent slug, got %q", got[0].Slug)
	}
	if got[1].Slug != "jira" {
		t.Errorf("unrelated slug must pass through, got %q", got[1].Slug)
	}
	if got[2].Slug != "redis-credentials" {
		t.Errorf("singular variant must canonicalize to the incumbent, got %q", got[2].Slug)
	}
	// Category must match: an entities candidate never rewrites a preference.
	mixed := []llmAtom{{Category: model.AtomCategoryEvents, Slug: "docs-language", Content: "x"}}
	gotM := canonicalizeAtoms(mixed, cands, nil)
	if gotM[0].Slug != "docs-language" {
		t.Errorf("cross-category candidate must not rewrite, got %q", gotM[0].Slug)
	}
}

// TestDedupAtoms (issue #27 task 4): near-verbatim restatements of an
// existing same-subject atom are suppressed at ingest; distinct facts and
// different subjects survive; in-batch duplicates are suppressed too.
func TestDedupAtoms(t *testing.T) {
	database := openExtractTestDB(t)
	defer database.Close()

	insertExistingAtom(t, database, "x1", model.AtomCategoryPreferences, "docs-language",
		"The user prefers documentation written in Chinese.")
	insertExistingAtom(t, database, "x2", model.AtomCategoryPreferences, "sql-format",
		"The user prefers SQL keywords in uppercase.")

	w := &Worker{db: database, log: testLogger()}

	parsed := []llmAtom{
		// Near-verbatim restatement (punctuation/ordering only) => suppressed.
		{Category: model.AtomCategoryPreferences, Slug: "docs-language",
			Content: "The user prefers documentation written in Chinese."},
		// Same words reordered => still near-verbatim => suppressed.
		{Category: model.AtomCategoryPreferences, Slug: "docs-language",
			Content: "Documentation written in Chinese is what the user prefers."},
		// Distinct fact, same subject => kept.
		{Category: model.AtomCategoryPreferences, Slug: "docs-language",
			Content: "Documentation language switch lives in the settings sidebar."},
		// Different subject entirely => kept.
		{Category: model.AtomCategoryEntities, Slug: "jenkins",
			Content: "Jenkins home directory is /var/lib/jenkins."},
		// In-batch duplicate => the second copy suppressed.
		{Category: model.AtomCategoryEntities, Slug: "jenkins",
			Content: "Jenkins home directory is /var/lib/jenkins."},
	}
	got, suppressed := w.dedupAtoms(context.Background(), parsed)

	if suppressed != 3 {
		t.Fatalf("want 3 suppressed (2 restatements + 1 in-batch dup), got %d", suppressed)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 survivors, got %d: %+v", len(got), got)
	}
	for _, a := range got {
		if strings.Contains(a.Content, "documentation") && a.Slug == "docs-language" && a.Content != "Documentation language switch lives in the settings sidebar." {
			t.Errorf("wrong survivor kept: %+v", a)
		}
	}
}

// TestCandidateSlugs (issue #27 task 2): relevant existing subjects are
// retrieved via memories FTS and grouped per category for the extract
// prompt.
func TestCandidateSlugs(t *testing.T) {
	database := openExtractTestDB(t)
	defer database.Close()

	insertExistingMemory(t, database, "m1", "entities", "jenkins",
		"Jenkins CI builds the starlink services on the build agent.")
	insertExistingMemory(t, database, "m2", "preferences", "sql-format",
		"The user prefers SQL keywords written in uppercase.")

	w := &Worker{db: database, log: testLogger()}
	got := w.candidateSlugs(context.Background(),
		`{"role":"user","content":"configure the jenkins build agent for starlink"}`)

	foundJenkins, foundSQL := false, false
	for _, c := range got {
		if c.Category == "entities" && c.Slug == "jenkins" {
			foundJenkins = true
		}
		if c.Category == "preferences" && c.Slug == "sql-format" {
			foundSQL = true
		}
	}
	if !foundJenkins {
		t.Errorf("jenkins candidate missing: %+v", got)
	}
	if foundSQL {
		t.Errorf("irrelevant candidate must not be retrieved for a jenkins session: %+v", got)
	}
}

func testLogger() *slog.Logger { return slog.Default() }
