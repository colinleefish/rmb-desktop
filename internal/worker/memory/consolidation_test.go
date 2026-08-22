package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
)

func jsonMarshal(s string) ([]byte, error) { return json.Marshal(s) }

func testCfg() config.PipelineConfig {
	cfg, err := config.Default()
	if err != nil {
		panic(err)
	}
	return cfg.Pipeline
}

func countRows(t *testing.T, database *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := database.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestMaterialityGate_IdenticalAtomsSkipRedistill (issue #27 task 1):
// with the atom-fingerprint gate, a bucket whose evidence did not change is
// NOT re-distilled — previously the source-scene gate re-distilled on every
// new session that restated an existing fact (profile hit ~8x/day).
func TestMaterialityGate_IdenticalAtomsSkipRedistill(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	insertPendingSession(t, database, "s1")
	insertPendingSession(t, database, "s2")
	insertAtom(t, database, "a1", "s1", model.AtomCategoryPreferences, "docs-language",
		"The user prefers documentation written in Chinese.")
	insertAtom(t, database, "a2", "s2", model.AtomCategoryPreferences, "docs-language",
		"The user prefers documentation written in Chinese.")

	distiller := &recordingDistiller{}
	w := testWorker(database, distiller)
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(distiller.calls); got != 1 {
		t.Fatalf("first rollup: want 1 distill (graduation + insert), got %d", got)
	}
	var version int
	if err := database.QueryRow(`SELECT version FROM memories WHERE uri='rmb://preferences/docs-language' AND superseded_at IS NULL`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("first rollup: version=%d want 1", version)
	}

	// New pending session that restates NOTHING for this subject: the
	// bucket's atom set is identical, so the materiality gate must skip the
	// distill entirely and leave the version alone.
	callsBefore := len(distiller.calls)
	insertPendingSession(t, database, "s3")
	insertAtom(t, database, "a3", "s3", model.AtomCategoryEvents, "2026-07-16-fix-tag-bug",
		"On 2026-07-16 the tag bug was fixed.")
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(distiller.calls) - callsBefore; got != 1 {
		// the one new call is the event bucket (exempt from gates)
		t.Fatalf("second rollup: want exactly 1 more distill (the event), got %d more", got)
	}
	for _, c := range distiller.calls[callsBefore:] {
		if c.Category == model.AtomCategoryPreferences && c.Slug == "docs-language" {
			t.Fatal("materiality gate failed: docs-language re-distilled with identical atom set")
		}
	}
	if err := database.QueryRow(`SELECT version FROM memories WHERE uri='rmb://preferences/docs-language' AND superseded_at IS NULL`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("materiality gate: version bumped to %d with no new evidence", version)
	}
}

// TestSemanticBodyGate_ParaphraseNoVersionBump (issue #27 task 6): when new
// atoms DO arrive but distill to the same information reworded, the body-diff
// gate suppresses the version bump (provenance fingerprint still refreshes).
func TestSemanticBodyGate_ParaphraseNoVersionBump(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	original := "- Speaks Chinese at home in Beijing.\n" +
		"- Works as an infrastructure engineer at HungryStudio.\n" +
		"- Uses a MacBook Pro and an iPhone 13 mini daily.\n" +
		"- Prefers concise technical answers with concrete commands."
	paraphrase := "- Speaks Chinese at home, based in Beijing.\n" +
		"- Works as an infrastructure engineer at HungryStudio.\n" +
		"- Uses a MacBook Pro and an iPhone 13 mini daily.\n" +
		"- Prefers concise technical answers with concrete commands."

	insertPendingSession(t, database, "s1")
	insertPendingSession(t, database, "s2")
	insertAtom(t, database, "a1", "s1", model.AtomCategoryProfile, "", original)
	insertAtom(t, database, "a2", "s2", model.AtomCategoryProfile, "", original)

	d := &fixedBodyDistiller{body: original}
	w := testWorker(database, d)
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}

	// New profile atom (a restatement with one extra clause) forces a
	// re-distill; the LLM now returns the paraphrased body — same
	// information, one synonym swap.
	insertPendingSession(t, database, "s3")
	insertAtom(t, database, "a3", "s3", model.AtomCategoryProfile, "", original+" Also likes tea.")
	d.body = paraphrase
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}

	var version int
	var body string
	var hash string
	if err := database.QueryRow(`SELECT version, body, COALESCE(source_atom_hash,'') FROM memories WHERE uri='rmb://profile' AND superseded_at IS NULL`).Scan(&version, &body, &hash); err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Fatalf("paraphrased body must not bump version, got version=%d", version)
	}
	if body != original {
		t.Fatalf("paraphrased body must not replace the active body")
	}
	if hash == "" {
		t.Fatal("provenance fingerprint must still refresh on paraphrase")
	}
	if n := countRows(t, database, `SELECT COUNT(*) FROM memories WHERE uri='rmb://profile' AND superseded_at IS NOT NULL`); n != 0 {
		t.Fatalf("paraphrase must not supersede the active row, got %d superseded rows", n)
	}
}

// fixedBodyDistiller always returns the current body (mutable between
// rollups to simulate a reworded re-distill).
type fixedBodyDistiller struct {
	mu   sync.Mutex
	body string
}

func (d *fixedBodyDistiller) DistillMemory(_ context.Context, category, slug, atomsJSON string, corrections []string, related []llm.RelatedEvent) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return `{"abstract":"a","body":` + strconvQuote(d.body) + `}`, nil
}

func strconvQuote(s string) string {
	// JSON-encode via json.Marshal for correctness.
	b, _ := jsonMarshal(s)
	return string(b)
}

// TestIncumbentMerge_PluralSlugVariant (issue #27 task 3): a NEW bucket slug
// that normalizes equal to an existing same-category active subject merges
// into the incumbent — bumping its version instead of minting a second
// subject uri. This is the doc-language/docs-language audit case.
func TestIncumbentMerge_PluralSlugVariant(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	insertPendingSession(t, database, "s1")
	insertPendingSession(t, database, "s2")
	// Incumbent subject (graduated across two sessions).
	insertAtom(t, database, "a1", "s1", model.AtomCategoryPreferences, "docs-language",
		"The user prefers documentation written in Chinese.")
	insertAtom(t, database, "a2", "s2", model.AtomCategoryPreferences, "docs-language",
		"The user prefers documentation written in Chinese.")
	w := testWorker(database, &recordingDistiller{})
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}

	// A later session coins the variant spelling "doc-language" (pre-P2.1
	// extract behavior). Two distinct sessions so the graduation bar passes.
	insertPendingSession(t, database, "s3")
	insertPendingSession(t, database, "s4")
	insertAtom(t, database, "a3", "s3", model.AtomCategoryPreferences, "doc-language",
		"Documentation for the user should be written in Chinese.")
	insertAtom(t, database, "a4", "s4", model.AtomCategoryPreferences, "doc-language",
		"Documentation for the user should be written in Chinese.")
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}

	if n := countRows(t, database, `SELECT COUNT(*) FROM memories WHERE uri='rmb://preferences/doc-language'`); n != 0 {
		t.Fatalf("variant slug must not mint a new memory, got %d rows", n)
	}
	var version int
	if err := database.QueryRow(`SELECT version FROM memories WHERE uri='rmb://preferences/docs-language' AND superseded_at IS NULL`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("incumbent must absorb the variant (version bump), got version=%d", version)
	}
}

// TestIncumbentMerge_CosinePath: with an embedder wired, a variant subject
// whose body embeds near-identically to an incumbent (but whose slug does
// NOT normalize equal) still merges into the incumbent.
func TestIncumbentMerge_CosinePath(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	insertPendingSession(t, database, "s1")
	insertPendingSession(t, database, "s2")
	insertAtom(t, database, "a1", "s1", model.AtomCategoryPreferences, "redis-credentials-storage",
		"Redis credentials live in the ops vault, never in repos.")
	insertAtom(t, database, "a2", "s2", model.AtomCategoryPreferences, "redis-credentials-storage",
		"Redis credentials live in the ops vault, never in repos.")
	w := NewWorker(database, &recordingDistiller{}, nil, testCfg(), nil, nil)
	if err := w.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Seed an embedding for the incumbent so the cos path can see it.
	embedIncumbent(t, database, "rmb://preferences/redis-credentials-storage", []float32{1, 0, 0, 0})

	// Variant: totally different slug tokens ("redis-secrets"), same subject.
	insertPendingSession(t, database, "s3")
	insertPendingSession(t, database, "s4")
	insertAtom(t, database, "a3", "s3", model.AtomCategoryPreferences, "redis-secrets",
		"Redis credentials live in the ops vault, never in repos.")
	insertAtom(t, database, "a4", "s4", model.AtomCategoryPreferences, "redis-secrets",
		"Redis credentials live in the ops vault, never in repos.")
	wEmbed := NewWorker(database, &recordingDistiller{}, stubEmbedder{vec: []float32{1, 0, 0, 0}}, testCfg(), nil, nil)
	if err := wEmbed.rollup(context.Background()); err != nil {
		t.Fatal(err)
	}

	if n := countRows(t, database, `SELECT COUNT(*) FROM memories WHERE uri='rmb://preferences/redis-secrets'`); n != 0 {
		t.Fatalf("cosine incumbent path: variant must not mint a new memory, got %d rows", n)
	}
	var version int
	if err := database.QueryRow(`SELECT version FROM memories WHERE uri='rmb://preferences/redis-credentials-storage' AND superseded_at IS NULL`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("cosine incumbent path: incumbent must bump to v2, got %d", version)
	}
}

// TestReduceInputCarriesAtomEvidence (issue #27 task 5): the multi-chunk
// reduce call must receive atom-level facts, never only distilled partials
// (rewrite-of-rewrite). The reduce payload carries both; facts are atoms.
func TestReduceInputCarriesAtomEvidence(t *testing.T) {
	database := openWorkerTestDB(t)
	defer database.Close()

	cfg := testCfg()
	cfg.L3MaxAtoms = 2 // force the multi-chunk reduce path

	atoms := make([]model.Atom, 5)
	for i := range atoms {
		atoms[i] = model.Atom{
			ID:       string(rune('a' + i)),
			Category: model.AtomCategoryPreferences,
			Content:  "Fact number " + string(rune('0'+i)) + " about reduce-path evidence.",
		}
	}
	bucket := Bucket{
		Category: model.AtomCategoryPreferences,
		Slug:     "reduce-path",
		URI:      "rmb://preferences/reduce-path",
		Atoms:    atoms,
	}

	distiller := &recordingDistiller{}
	w := NewWorker(database, distiller, nil, cfg, nil, nil)
	if _, err := w.distillBucket(context.Background(), bucket, nil); err != nil {
		t.Fatal(err)
	}

	// 5 atoms at max 2 per chunk = 3 chunk distills + 1 reduce call.
	if len(distiller.calls) != 4 {
		t.Fatalf("want 3 chunk distills + 1 reduce, got %d calls", len(distiller.calls))
	}
	reduce := distiller.calls[3].AtomsJSON
	if !strings.Contains(reduce, `"uri":"rmb://atoms/a"`) {
		t.Errorf("reduce input must carry atom-level facts, got: %.200s", reduce)
	}
	if !strings.Contains(reduce, `"partials"`) {
		t.Errorf("reduce input should keep partials as secondary context")
	}
	if !strings.Contains(reduce, "reduce-path evidence") {
		t.Errorf("reduce input must carry atom content, not only distilled text")
	}
}

type stubEmbedder struct {
	vec []float32
}

func (s stubEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return [][]float32{s.vec}, nil
}

func embedIncumbent(t *testing.T, database *sql.DB, memoryURI string, vec []float32) {
	t.Helper()
	// sqlite-vec LE float32 blob, matching recall.DecodeVecFloat32.
	blob := make([]byte, 0, len(vec)*4)
	for _, f := range vec {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], math.Float32bits(f))
		blob = append(blob, buf[:]...)
	}
	if _, err := database.Exec(`UPDATE memories SET embedding = ? WHERE uri = ? AND superseded_at IS NULL`, blob, memoryURI); err != nil {
		t.Fatal(err)
	}
}
