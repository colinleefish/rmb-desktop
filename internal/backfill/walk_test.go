package backfill

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/inspect"
)

// TestProvenanceWalkableEndToEnd verifies the acceptance: after backfill, a
// memory's source_scene_uris resolves to a scene whose meta exposes both
// session identifiers, so `ls rmb://sessions/<scene's session_id>/` walks the
// pyramid down to turns+atoms.
func TestProvenanceWalkableEndToEnd(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	nowMS := time.Now().UTC().UnixMilli()
	sessionID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	sceneID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	memID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	memURI := "rmb://events/2026-08-21-openresty-resolver"

	if _, err := database.Exec(`INSERT INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES (?, 'starlink-openresty', '', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, embedding, created_at, updated_at)
		VALUES (?, ?, '', 'openresty resolver', 'resolver ip 10.0.0.1 upstreams proc', '["atom-1"]', ?, ?, ?)`,
		sceneID, sessionID, blob([]float32{1, 0, 0, 0}), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
		VALUES ('turn-1', ?, '[]', ?, 'done')`, sessionID, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO atoms (id, session_id, category, priority, content, created_at, updated_at)
		VALUES ('atom-1', ?, 'events', 50, 'openresty resolver ip', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO memories (id, uri, category, version, abstract, body, source_scene_uris, source_correction_uris, embedding, created_at, updated_at)
		VALUES (?, ?, 'events', 1, 's', 'resolver ip 10.0.0.1 upstreams proc', '[]', '[]', ?, ?, ?)`,
		memID, memURI, blob([]float32{1, 0.05, 0, 0}), nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	// Backfill the empty provenance.
	stats, err := BackfillProvenance(context.Background(), database, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.MemoriesLinked != 1 {
		t.Fatalf("linked=%d want 1", stats.MemoriesLinked)
	}

	// Walk: memory meta → scene uri.
	svc := inspect.NewService(database)
	var mBuf bytes.Buffer
	if err := svc.Meta(context.Background(), memURI, &mBuf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mBuf.String(), `"source_scene_uris"`) || !strings.Contains(mBuf.String(), "rmb://scenes/"+sceneID) {
		t.Fatalf("memory meta missing backfilled scene: %s", mBuf.String())
	}

	// Scene meta → both session identifiers.
	var sBuf bytes.Buffer
	if err := svc.Meta(context.Background(), "rmb://scenes/"+sceneID, &sBuf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"session_id": "` + sessionID + `"`, `"session_key": "starlink-openresty"`} {
		if !strings.Contains(sBuf.String(), want) {
			t.Fatalf("scene meta missing %s: %s", want, sBuf.String())
		}
	}

	// ls session by the scene's session_id → turns + atoms.
	var lsBuf bytes.Buffer
	if err := svc.Ls(context.Background(), "rmb://sessions/"+sessionID+"/", &lsBuf); err != nil {
		t.Fatalf("ls by session_id: %v", err)
	}
	for _, want := range []string{"turn-1", "atom-1"} {
		if !strings.Contains(lsBuf.String(), want) {
			t.Fatalf("session ls missing %s: %q", want, lsBuf.String())
		}
	}
}
