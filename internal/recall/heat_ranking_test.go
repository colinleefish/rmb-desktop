package recall_test

import (
	"context"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/recall"
)

// TestHeatRanking_promotesHotOverEqualOpponent verifies that when heat ranking
// is enabled, a high-heat memory with (slightly) WORSE relevance overtakes a
// near-equal relevance opponent that is hotter... i.e. the boost breaks a
// near-tie. Both memories have NULL embeddings so the vector leg is empty and
// FTS contributes RRF scores that differ by a single rank (~0.00008) — far
// less than the heat boost (α·log(1+50)≈0.0014). Enabled ⇒ hot wins;
// disabled or --no-boost ⇒ cold (lexicographically-earlier) wins.
func TestHeatRanking_promotesHotOverEqualOpponent(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	// Cold memory first → lower rowid → FTS ranks it ahead of the hot one
	// (identical matched tokens), so without boost the cold memory wins the
	// near-tie. Extra filler words push the hot memory's bm25 slightly lower
	// so the ordering is deterministic.
	const (
		coldURI = "rmb://events/a-cold"
		hotURI  = "rmb://events/z-hot"
	)
	insert := func(id, uri, abs string, updatedMS int64) {
		t.Helper()
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
			VALUES (?, ?, 'events', 1, ?, '', '[]', '[]', ?, ?)`,
			id, uri, abs, updatedMS, updatedMS); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories_fts(rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, '')`, id, abs); err != nil {
			t.Fatal(err)
		}
	}
	insert("11111111-1111-4111-8111-111111111111", coldURI,
		"kubectl apply deployment yaml", nowMS)
	insert("22222222-2222-4222-8222-222222222222", hotURI,
		"kubectl apply deployment yaml with considerably more filler words that push this memory's bm25 rank down behind the cold one deterministically for this test",
		nowMS)

	// Heat for the hot memory only (cold has none → heat 0).
	highHeat := 50.0
	if _, err := database.Exec(`
		INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, heat, updated_at, last_use_at)
		VALUES (?, 0, 1, 0, ?, ?, ?)`, hotURI, highHeat, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	svc := recall.NewService(database)
	// Non-nil embedder so the vector leg is attempted; with NULL embeddings it
	// returns nothing, leaving a pure FTS (RRF-scaled) near-tie.
	embed := recall.QueryEmbedder(func(_ context.Context, q string) ([]float32, error) {
		return []float32{1, 0, 0, 0}, nil
	})

	first := func(opts ...recall.SearchOption) string {
		m, err := svc.Search(context.Background(), embed, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{}, opts...)
		if err != nil {
			t.Fatal(err)
		}
		if len(m) == 0 {
			return ""
		}
		return m[0].URI
	}

	if got := first(); got != coldURI {
		t.Fatalf("disabled: first = %s, want cold %s (boost must be inert by default)", got, coldURI)
	}
	if got := first(recall.WithHeatRanking(true)); got != hotURI {
		t.Fatalf("enabled: first = %s, want hot %s promoted over the near-equal cold opponent", got, hotURI)
	}
	// --no-boost cancels the forced enable: back to pure relevance.
	if got := first(recall.WithHeatRanking(true), recall.WithNoBoost()); got != coldURI {
		t.Fatalf("no-boost: first = %s, want cold %s", got, coldURI)
	}
	// Explicit WithHeatRanking(false) equals WithNoBoost.
	if got := first(recall.WithHeatRanking(false)); got != coldURI {
		t.Fatalf("WithHeatRanking(false): first = %s, want cold %s", got, coldURI)
	}
}

// TestHeatRanking_ageDecayPromotesFresh verifies the cold-start novelty term:
// with equal relevance and equal heat (0), a fresh memory gets a bigger β
// boost than a stale one and is therefore promoted when enabled.
func TestHeatRanking_ageDecayPromotesFresh(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	day := int64(24 * time.Hour.Milliseconds())
	const (
		staleURI = "rmb://events/a-stale"
		freshURI = "rmb://events/z-fresh"
	)
	insert := func(id, uri, abs string, updatedMS int64) {
		t.Helper()
		if _, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
			VALUES (?, ?, 'events', 1, ?, '', '[]', '[]', ?, ?)`,
			id, uri, abs, updatedMS, updatedMS); err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(`
			INSERT INTO memories_fts(rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, '')`, id, abs); err != nil {
			t.Fatal(err)
		}
	}
	filler := " with considerably more filler words that push this memory's bm25 rank down deterministically for this test"
	insert("11111111-1111-4111-8111-111111111111", staleURI, "kubectl apply deployment yaml"+filler, nowMS-400*day)
	insert("22222222-2222-4222-8222-222222222222", freshURI, "kubectl apply deployment yaml"+filler, nowMS-day)

	svc := recall.NewService(database)
	embed := recall.QueryEmbedder(func(_ context.Context, q string) ([]float32, error) {
		return []float32{1, 0, 0, 0}, nil
	})
	first := func(opts ...recall.SearchOption) string {
		m, err := svc.Search(context.Background(), embed, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{}, opts...)
		if err != nil {
			t.Fatal(err)
		}
		if len(m) == 0 {
			return ""
		}
		return m[0].URI
	}

	// Disabled: stale (lexicographically earlier) wins the tie by URI.
	if got := first(); got != staleURI {
		t.Fatalf("disabled: first = %s, want stale %s", got, staleURI)
	}
	// Enabled: the fresh memory's larger novelty boost promotes it.
	if got := first(recall.WithHeatRanking(true)); got != freshURI {
		t.Fatalf("enabled: first = %s, want fresh %s promoted by the age novelty term", got, freshURI)
	}
	if got := first(recall.WithHeatRanking(true), recall.WithNoBoost()); got != staleURI {
		t.Fatalf("no-boost: first = %s, want stale %s", got, staleURI)
	}
}

// TestHeatRanking_disabledMatchesNoBoost asserts the default (off) path is a
// no-op relative to --no-boost: identical result lists prove the boost block
// is byte-for-byte inert when not enabled.
func TestHeatRanking_disabledMatchesNoBoost(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	uri := "rmb://entities/kubernetes-hot"
	if _, err := database.Exec(`
		INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
		VALUES ('33333333-3333-4333-8333-333333333333', ?, 'entities', 1, 'k8s', 'kubectl apply deployment yaml', '[]', '[]', ?, ?)`,
		uri, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO memories_fts(rowid, abstract, body)
		VALUES ((SELECT rowid FROM memories WHERE id = '33333333-3333-4333-8333-333333333333'), 'k8s', 'kubectl apply deployment yaml')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, heat, updated_at, last_use_at)
		VALUES (?, 0, 5, 0, 900, ?, ?)`, uri, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	svc := recall.NewService(database)
	uris := func(opts ...recall.SearchOption) []string {
		m, err := svc.Search(context.Background(), nil, "kubectl deployment", 5, []string{"memory"}, recall.TimeWindow{}, opts...)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(m))
		for _, x := range m {
			out = append(out, x.URI)
		}
		return out
	}
	def := uris()
	noBoost := uris(recall.WithNoBoost())
	off := uris(recall.WithHeatRanking(false))

	// Default (off) must be byte-identical to an explicit no-boost even though
	// this memory has very high heat (900) — the boost path is not consulted.
	if len(def) != len(noBoost) {
		t.Fatalf("default %v vs no-boost %v", def, noBoost)
	}
	for i := range def {
		if def[i] != noBoost[i] || def[i] != off[i] {
			t.Fatalf("output differs (default=%v no-boost=%v off=%v)", def, noBoost, off)
		}
	}
}
