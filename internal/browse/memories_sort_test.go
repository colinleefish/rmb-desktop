package browse_test

import (
	"context"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/browse"
	"github.com/colinleefish/rmb-desktop/internal/recallstats"
)

// TestListMemoriesSortByRecallStats covers the recall-stats sort keys added
// for the sortable Search/Cat/Meta columns: recall_stats is a separate table
// (1:1 by uri, LEFT JOINed), so sorting by search/cat/meta must include
// memories that were never recalled (COALESCE 0).
func TestListMemoriesSortByRecallStats(t *testing.T) {
	database := openTestDB(t)
	svc := browse.NewService(database, recallstats.NewService(database))

	mustExec(t, database, `INSERT INTO memories (id, uri, category, slug, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at) VALUES
		('m1', 'rmb://entities/a', 'entities', 'a', 1, 'A', 'body a', '[]', '[]', 1, 1),
		('m2', 'rmb://entities/b', 'entities', 'b', 1, 'B', 'body b', '[]', '[]', 2, 2),
		('m3', 'rmb://entities/c', 'entities', 'c', 1, 'C', 'body c', '[]', '[]', 3, 3),
		('m4', 'rmb://entities/d', 'entities', 'd', 1, 'D', 'body d', '[]', '[]', 4, 4)`)
	mustExec(t, database, `INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, updated_at) VALUES
		('rmb://entities/a', 5, 2, 9, 1),
		('rmb://entities/b', 1, 7, 3, 1),
		('rmb://entities/c', 3, 4, 1, 1)`) // d has no recall_stats row → all 0

	slugs := func(page browse.Page[browse.MemoryJSON]) []string {
		out := make([]string, 0, len(page.Items))
		for _, it := range page.Items {
			if it.Slug != nil {
				out = append(out, *it.Slug)
			}
		}
		return out
	}
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	cases := []struct {
		sort, order string
		want        []string
	}{
		{"search", "desc", []string{"a", "c", "b", "d"}}, // 5,3,1,0
		{"search", "asc", []string{"d", "b", "c", "a"}},  // 0,1,3,5
		{"cat", "desc", []string{"b", "c", "a", "d"}},    // 7,4,2,0
		{"meta", "desc", []string{"a", "b", "c", "d"}},   // 9,3,1,0
	}
	for _, tc := range cases {
		page, err := svc.ListMemories(context.Background(), browse.ListParams{
			Limit: 10, Sort: tc.sort, Order: tc.order,
		})
		if err != nil {
			t.Fatalf("ListMemories sort=%s: %v", tc.sort, err)
		}
		got := slugs(page)
		if !eq(got, tc.want) {
			t.Errorf("sort=%s order=%s: got %v want %v", tc.sort, tc.order, got, tc.want)
		}
	}

	// Category filter must still work alongside the join.
	page, err := svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 10, Category: "entities", Sort: "search", Order: "desc",
	})
	if err != nil {
		t.Fatalf("ListMemories with category: %v", err)
	}
	if len(page.Items) != 4 {
		t.Errorf("category filter with join: got %d items want 4", len(page.Items))
	}

	// Query filter (matches uri/abstract/body/slug) must work alongside join.
	page, err = svc.ListMemories(context.Background(), browse.ListParams{
		Limit: 10, Query: "body a", Sort: "search", Order: "desc",
	})
	if err != nil {
		t.Fatalf("ListMemories with query: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Slug == nil || *page.Items[0].Slug != "a" {
		t.Errorf("query filter with join: got %+v want [a]", page.Items)
	}
}
