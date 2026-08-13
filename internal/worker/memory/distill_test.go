package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/model"
)

func TestDistillBucketSingleAtomSkipsLLM(t *testing.T) {
	w := &Worker{} // single-atom path must not touch w.llm
	atom := model.Atom{Content: "The user uses Atlas for schema comparison."}
	bucket := Bucket{
		Category: "preferences",
		Slug:     "atlas",
		URI:      "rmb://preferences/atlas",
		Atoms:    []model.Atom{atom},
	}

	pm, err := w.distillBucket(context.Background(), bucket, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pm.Body != "The user uses Atlas for schema comparison." {
		t.Fatalf("body=%q", pm.Body)
	}
	if pm.Abstract == "" {
		t.Fatal("empty abstract")
	}
}

func TestDistillBucketSingleAtomTruncatesAbstract(t *testing.T) {
	w := &Worker{}
	long := strings.Repeat("x", 500)
	atom := model.Atom{Content: long}
	bucket := Bucket{
		Category: "entities",
		Slug:     "big",
		URI:      "rmb://entities/big",
		Atoms:    []model.Atom{atom},
	}

	pm, err := w.distillBucket(context.Background(), bucket, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pm.Body != long {
		t.Fatalf("body should stay full, got %d chars", len(pm.Body))
	}
	if len([]rune(pm.Abstract)) > 200 {
		t.Fatalf("abstract truncated to %d runes, want <= 200", len([]rune(pm.Abstract)))
	}
}

func TestDistillBucketSingleAtomEmptyContent(t *testing.T) {
	w := &Worker{}
	bucket := Bucket{
		Category: "entities",
		Slug:     "empty",
		URI:      "rmb://entities/empty",
		Atoms:    []model.Atom{{Content: "   "}},
	}
	if _, err := w.distillBucket(context.Background(), bucket, nil); err == nil {
		t.Fatal("expected error for empty single-atom content")
	}
}
