package recall_test

import (
	"context"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/recall"
)

func TestFTSEventLinks(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	insert := func(id, uri, category string, superseded bool, abstract, body string) {
		t.Helper()
		var sup any
		if superseded {
			sup = nowMS
		}
		_, err := database.Exec(`
			INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, source_correction_uris, created_at, updated_at)
			VALUES (?, ?, ?, 1, ?, ?, ?, '[]', '[]', ?, ?)`,
			id, uri, category, sup, abstract, body, nowMS, nowMS)
		if err != nil {
			t.Fatal(err)
		}
		_, err = database.Exec(`
			INSERT INTO memories_fts(rowid, abstract, body)
			VALUES ((SELECT rowid FROM memories WHERE id = ?), ?, ?)`, id, abstract, body)
		if err != nil {
			t.Fatal(err)
		}
	}

	insert("e1", "rmb://events/2026-07-13-starlink-hs99-vip-500-bug", "events", false,
		"starlink hs99 vip 500 bug", "get_diff_tag_from_tag_base_editer dict-vs-list bug caused 500s on tag base editer")
	insert("e2", "rmb://events/2026-07-16-soft-delete-one-tag-solutions", "events", false,
		"soft delete one tag solutions", "soft-deleted 29800 of 29867 one-tag-diff solutions in lhh_blockblast_client")
	insert("e3", "rmb://events/2026-07-18-superseded-event", "events", true,
		"superseded event", "old superseded event about tag solutions that must not be returned")
	insert("x1", "rmb://entities/one-tag-solution", "entities", false,
		"one tag solution entity", "one-tag-diff solution entity also mentions tag solutions")
	insert("self", "rmb://events/2026-07-16-soft-delete-one-tag-solutions-dup", "events", false,
		"duplicate slug probe", "self event must be excluded")

	matches, err := recall.FTSEventLinks(context.Background(), database,
		"soft delete tag solutions", "rmb://events/2026-07-16-soft-delete-one-tag-solutions-dup", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected event link hits")
	}
	for _, m := range matches {
		if m.Tier != "events" {
			t.Errorf("tier=%s want events", m.Tier)
		}
		if m.URI == "rmb://events/2026-07-16-soft-delete-one-tag-solutions-dup" {
			t.Error("excluded uri must not be returned")
		}
		if m.URI == "rmb://events/2026-07-18-superseded-event" {
			t.Error("superseded memory must not be returned")
		}
		if m.URI == "rmb://entities/one-tag-solution" {
			t.Error("non-event memories must not be returned")
		}
	}
	if matches[0].URI != "rmb://events/2026-07-16-soft-delete-one-tag-solutions" {
		t.Errorf("best match uri=%s", matches[0].URI)
	}

	// OR semantics: a single shared token still links (bm25 ranks it).
	one, err := recall.FTSEventLinks(context.Background(), database, "hs99", "self-x", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) == 0 || one[0].URI != "rmb://events/2026-07-13-starlink-hs99-vip-500-bug" {
		t.Errorf("single-token OR match failed: %+v", one)
	}
}

func TestEscapeFTSQueryAny(t *testing.T) {
	got := recall.EscapeFTSQueryAny(`cluster admin "toolbox`)
	want := `"cluster" OR "admin" OR """toolbox"`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if recall.EscapeFTSQueryAny("   ") != `""` {
		t.Fatal("empty query must escape to empty phrase")
	}
}
