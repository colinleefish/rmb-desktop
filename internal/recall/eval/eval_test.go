package eval_test

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/recall/eval"
)

// TestGoldenRegression runs the committed golden set against the committed
// fixture snapshot and asserts the v1 baseline gates. This is the offline-safe
// subset wired into CI.
func TestGoldenRegression(t *testing.T) {
	dir := t.TempDir()
	scratch := filepath.Join(dir, "scratch.db")
	fix, err := eval.LoadFixture("testdata/golden_fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := eval.LoadGolden("golden.yaml")
	if err != nil {
		t.Fatal(err)
	}
	database, err := fix.BuildDB(scratch)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	report, err := eval.Run(context.Background(), database, golden)
	if err != nil {
		t.Fatal(err)
	}

	for _, q := range report.Questions {
		if !q.Hit {
			t.Logf("MISS  %-28s %s", q.ID, q.Query)
		}
	}
	t.Logf("recall@5=%.3f dup-rate=%.3f recency-precision=%.3f (%d/%d)",
		report.RecallAt5, report.DupRate, report.RecencyPrecision, report.Passed, report.TotalQuestions)

	if !report.GatesOK(&golden.Gates) {
		t.Fatalf("gates not met: %+v (report %+v)", golden.Gates, report)
	}
}

func TestHashEmbedDeterministic(t *testing.T) {
	a := eval.HashEmbed("hello world k8s deployment")
	b := eval.HashEmbed("hello world k8s deployment")
	if len(a) != eval.EmbedDim || len(b) != eval.EmbedDim {
		t.Fatalf("bad dim")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("hash embed not deterministic")
		}
	}
	// related text should be more similar than unrelated text
	related := eval.HashEmbed("hello k8s deployment")
	unrelated := eval.HashEmbed("pasta recipe alfredo cream")
	d := cosine(a, related)
	d2 := cosine(a, unrelated)
	if d <= d2 {
		t.Fatalf("expected lexical similarity to dominate; got %f vs %f", d, d2)
	}
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
