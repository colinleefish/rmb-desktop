package browse_test

import (
	"context"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/browse"
	"github.com/colinleefish/rmb-desktop/internal/recallstats"
)

// TestSearchWhereDoesNotBypassFilters is a regression test for the SQL
// precedence bug where the unparenthesized OR chain from searchWhere escaped
// the AND-ed filters (superseded_at, category). Same pattern leaked skills.
func TestSearchWhereDoesNotBypassFilters(t *testing.T) {
	database := openTestDB(t)
	svc := browse.NewService(database, recallstats.NewService(database))
	now := time.Now().UTC().UnixMilli()

	// Two versions of the same memory: v1 superseded, v2 active.
	mustExec(t, database, `INSERT INTO memories (id, uri, category, slug, version, abstract, body, source_scene_uris, source_correction_uris, superseded_at, created_at, updated_at) VALUES
		('m-old', 'rmb://entities/rmb', 'entities', 'rmb', 1, 'old abstract', 'old body', '[]', '[]', ?, 1, 1),
		('m-new', 'rmb://entities/rmb', 'entities', 'rmb', 2, 'new abstract', 'new body', '[]', '[]', NULL, 2, 2),
		('m-other', 'rmb://events/x', 'events', 'x', 1, 'unrelated', 'unrelated', '[]', '[]', NULL, 3, 3)`,
		now)

	// A query whose LIKE terms match the superseded row too ("rmb" is in
	// its slug/body) must still return only the active version, and must
	// not pull rows from other categories.
	page, err := svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 25, Query: "rmb", Category: "entities", Sort: "updated", Order: "desc",
	})
	if err != nil {
		t.Fatalf("ListMemories with query: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("query must return only the active memory in category: got total=%d items=%d want 1", page.Total, len(page.Items))
	}
	if page.Items[0].Version != 2 {
		t.Errorf("got version %d want 2 (active)", page.Items[0].Version)
	}

	// Skills: superseded skills must stay hidden regardless of query.
	mustExec(t, database, `INSERT INTO skills (id, uri, slug, name, description, version, bundle_sha256, superseded_at, created_at, updated_at) VALUES
		('s-old', 'rmb://skills/demo', 'demo', 'Demo', 'old', 1, 'x', ?, 1, 1),
		('s-new', 'rmb://skills/demo', 'demo', 'Demo', 'new', 2, 'x', NULL, 2, 2),
		('s-plain', 'rmb://skills/plain', 'plain', 'Plain', 'zzz', 1, 'x', NULL, 3, 3)`,
		now)
	skillPage, err := svc.ListSkills(context.Background(), browse.ListParams{Limit: 25, Query: "demo"})
	if err != nil {
		t.Fatalf("ListSkills with query: %v", err)
	}
	if skillPage.Total != 1 || len(skillPage.Items) != 1 || skillPage.Items[0].Version != 2 {
		t.Fatalf("skills query must return only the active skill: got total=%d want 1", skillPage.Total)
	}
}

// TestSearchIgnoresURIScheme is a regression test for the search-uselessness
// bug: the uri column was matched raw, so any query containing a substring of
// the ubiquitous "rmb://" scheme prefix (e.g. "rmb") matched every row in the
// table — 1858 of 1977 entity matches were scheme-only. Now the uri is
// searched via substr(uri, 7) (path after the scheme) and a pasted full URI
// has its scheme stripped, so it still finds the row.
func TestSearchIgnoresURIScheme(t *testing.T) {
	database := openTestDB(t)
	svc := browse.NewService(database, recallstats.NewService(database))

	mustExec(t, database, `INSERT INTO memories (id, uri, category, slug, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at) VALUES
		('m-rmb', 'rmb://entities/rmb', 'entities', 'rmb', 2, 'the memory system', 'the memory system', '[]', '[]', 1, 1),
		('m-mj', 'rmb://entities/lsx-blockmj', 'entities', 'lsx-blockmj', 1, 'mahjong game', 'mahjong game', '[]', '[]', 2, 2),
		('m-ev', 'rmb://events/2026-08-09-rmb-branch-cleanup', 'events', '2026-08-09-rmb-branch-cleanup', 1, 'branch cleanup', 'branch cleanup', '[]', '[]', 3, 3)`)

	// q=rmb must NOT match every row via the "rmb://" scheme: only the slug
	// hit and the event whose path genuinely contains "rmb".
	page, err := svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 25, Query: "rmb",
	})
	if err != nil {
		t.Fatalf("ListMemories q=rmb: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("q=rmb total: got %d want 2 (scheme-only rows must not match); got %v", page.Total, uris(page))
	}

	// Pasted full URI still resolves to the row (scheme stripped, path matched).
	page, err = svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 25, Query: "RMB://entities/rmb",
	})
	if err != nil {
		t.Fatalf("ListMemories pasted uri: %v", err)
	}
	if page.Total != 1 || page.Items[0].URI != "rmb://entities/rmb" {
		t.Fatalf("pasted uri: got total=%d uris=%v want exactly rmb://entities/rmb", page.Total, uris(page))
	}

	// Query that is only the scheme is treated as no query at all.
	page, err = svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 25, Query: "rmb://",
	})
	if err != nil {
		t.Fatalf("ListMemories scheme-only query: %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("scheme-only query: got total=%d want 3 (acts as empty query)", page.Total)
	}

	// Path fragments still work, e.g. scope or date.
	page, err = svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 25, Query: "2026-08-09",
	})
	if err != nil {
		t.Fatalf("ListMemories date query: %v", err)
	}
	if page.Total != 1 || page.Items[0].URI != "rmb://events/2026-08-09-rmb-branch-cleanup" {
		t.Fatalf("date query: got total=%d uris=%v", page.Total, uris(page))
	}

	// Skills: same scheme rule — q=rmb must not match every skill.
	mustExec(t, database, `INSERT INTO skills (id, uri, slug, name, description, version, bundle_sha256, created_at, updated_at) VALUES
		('s-rmb', 'rmb://skills/rmb-tips', 'rmb-tips', 'RMB Tips', 'how to use', 1, 'x', 1, 1),
		('s-other', 'rmb://skills/plain', 'plain', 'Plain', 'zzz', 1, 'x', 2, 2)`)
	skillPage, err := svc.ListSkills(context.Background(), browse.ListParams{Limit: 25, Query: "rmb"})
	if err != nil {
		t.Fatalf("ListSkills q=rmb: %v", err)
	}
	if skillPage.Total != 1 || skillPage.Items[0].Slug != "rmb-tips" {
		t.Fatalf("skills q=rmb: got total=%d want 1 (rmb-tips only)", skillPage.Total)
	}
}

func uris(page browse.Page[browse.MemoryJSON]) []string {
	out := make([]string, 0, len(page.Items))
	for _, it := range page.Items {
		out = append(out, it.URI)
	}
	return out
}
