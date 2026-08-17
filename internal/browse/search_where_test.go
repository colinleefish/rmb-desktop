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
// the AND-ed filters. A query matching every "rmb://..." uri used to return
// superseded memory versions (e.g. rmb://entities/rmb v65, v74 alongside the
// active version) and ignore the category filter. Same pattern for skills
// (superseded_at) and session-scoped atoms/scenes (session_id).
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

	// A query matching every uri ("rmb://") must not leak superseded rows
	// nor rows from other categories.
	page, err := svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 25, Query: "rmb://", Category: "entities", Sort: "updated", Order: "desc",
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
		('s-new', 'rmb://skills/demo', 'demo', 'Demo', 'new', 2, 'x', NULL, 2, 2)`,
		now)
	skillPage, err := svc.ListSkills(context.Background(), browse.ListParams{Limit: 25, Query: "demo"})
	if err != nil {
		t.Fatalf("ListSkills with query: %v", err)
	}
	if skillPage.Total != 1 || len(skillPage.Items) != 1 || skillPage.Items[0].Version != 2 {
		t.Fatalf("skills query must return only the active skill: got total=%d want 1", skillPage.Total)
	}
}
