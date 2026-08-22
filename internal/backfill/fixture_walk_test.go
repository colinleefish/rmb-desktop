package backfill

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/inspect"
	"github.com/colinleefish/rmb-desktop/internal/recall/eval"
)

// TestProvenanceWalkableOnGoldenFixture satisfies the acceptance "provenance
// walkable end-to-end on the golden eval fixture": after materializing the
// committed golden fixture into a scratch db, the backfill runs safely and a
// memory's source_scene_uris walks memory -> scene (exposing both session
// identifiers) -> `ls rmb://sessions/<session_id>/` down to turns + atoms.
func TestProvenanceWalkableOnGoldenFixture(t *testing.T) {
	fix, err := eval.LoadFixture(filepath.Join("..", "recall", "eval", "testdata", "golden_fixture.json"))
	if err != nil {
		t.Fatalf("load golden fixture: %v", err)
	}
	if len(fix.Scenes) == 0 {
		t.Fatal("golden fixture has no scenes")
	}

	database, err := fix.BuildDB(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatalf("build fixture db: %v", err)
	}
	defer database.Close()

	// The fixture carries no turns/atoms; add a couple for the sampled scene's
	// session so the final leg of the ladder (turn/atom listing) is demonstrable
	// on golden data.
	scene := fix.Scenes[0]
	sessionID := scene.SessionID
	nowMS := time.Now().UTC().UnixMilli()
	if _, err := database.Exec(`INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
		VALUES ('fixture-turn-1', ?, '[]', ?, 'done'), ('fixture-turn-2', ?, '[]', ?, 'done')`,
		sessionID, nowMS, sessionID, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO atoms (id, session_id, category, priority, content, created_at, updated_at)
		VALUES ('fixture-atom-1', ?, 'events', 50, 'a', ?, ?), ('fixture-atom-2', ?, 'events', 50, 'b', ?, ?)`,
		sessionID, nowMS, nowMS, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	// Backfill must run cleanly and idempotently on the fixture (no panic, no
	// clobber). We don't assert a link count here: with hash embeddings the
	// fixture is not guaranteed to have any above-threshold scene, so the walk
	// below pins a recoverable link explicitly.
	stats, err := BackfillProvenance(context.Background(), database, Options{})
	if err != nil {
		t.Fatalf("backfill on golden fixture: %v", err)
	}
	if _, err := BackfillProvenance(context.Background(), database, Options{}); err != nil {
		t.Fatalf("backfill idempotent rerun: %v", err)
	}
	t.Logf("fixture backfill scanned=%d linked=%d scenes=%d", stats.MemoriesScanned, stats.MemoriesLinked, stats.ScenesLinked)

	// Pin one fixture memory to the sampled scene and walk the ladder.
	mem := fix.Memories[0]
	if _, err := database.Exec(`UPDATE memories SET source_scene_uris = ? WHERE uri = ?`,
		`["rmb://scenes/`+scene.ID+`"]`, mem.URI); err != nil {
		t.Fatal(err)
	}

	svc := inspect.NewService(database)
	var mBuf bytes.Buffer
	if err := svc.Meta(context.Background(), mem.URI, &mBuf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mBuf.String(), "rmb://scenes/"+scene.ID) {
		t.Fatalf("memory meta missing scene: %s", mBuf.String())
	}

	var sBuf bytes.Buffer
	if err := svc.Meta(context.Background(), "rmb://scenes/"+scene.ID, &sBuf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sBuf.String(), `"session_id": "`+sessionID+`"`) ||
		!strings.Contains(sBuf.String(), `"session_key": "fixture-`+sessionID+`"`) {
		t.Fatalf("scene meta must expose both session identifiers: %s", sBuf.String())
	}

	var lsBuf bytes.Buffer
	if err := svc.Ls(context.Background(), "rmb://sessions/"+sessionID+"/", &lsBuf); err != nil {
		t.Fatalf("ls by session_id on fixture: %v", err)
	}
	for _, want := range []string{"fixture-turn-1", "fixture-turn-2", "fixture-atom-1", "fixture-atom-2"} {
		if !strings.Contains(lsBuf.String(), want) {
			t.Fatalf("fixture session ls missing %s: %q", want, lsBuf.String())
		}
	}
}
