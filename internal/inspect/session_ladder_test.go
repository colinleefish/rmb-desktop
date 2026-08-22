package inspect

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestSessionLadder_KeyAndIDUnification verifies the pyramid can be walked
// downward (RC7): `ls rmb://sessions/<key>/` AND `ls rmb://sessions/<id>/`
// both resolve, and cat/meta accept both forms too.
func TestSessionLadder_KeyAndIDUnification(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	nowMS := time.Now().UTC().UnixMilli()
	sessionID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err := s.db.Exec(`
		INSERT INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES (?, 'focused', 'about focused sessions', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
		VALUES ('turn-1', ?, '[]', ?, 'done')`, sessionID, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO atoms (id, session_id, category, priority, content, created_at, updated_at)
		VALUES ('atom-1', ?, 'events', 50, 'a fact', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	// ls by session_key
	var byKey bytes.Buffer
	if err := s.Ls(ctx, "rmb://sessions/focused/", &byKey); err != nil {
		t.Fatalf("ls by key: %v", err)
	}
	// ls by session_id
	var byID bytes.Buffer
	if err := s.Ls(ctx, "rmb://sessions/"+sessionID+"/", &byID); err != nil {
		t.Fatalf("ls by id: %v", err)
	}
	if byKey.String() != byID.String() {
		t.Fatalf("key/id ls differ:\nkey=%q\nid =%q", byKey.String(), byID.String())
	}
	for _, want := range []string{"turn-1", "atom-1"} {
		if !strings.Contains(byID.String(), want) {
			t.Fatalf("ls by id missing %s: %q", want, byID.String())
		}
	}

	// cat by both forms
	var c1, c2 bytes.Buffer
	if err := s.Cat(ctx, "rmb://sessions/focused", &c1); err != nil {
		t.Fatalf("cat by key: %v", err)
	}
	if err := s.Cat(ctx, "rmb://sessions/"+sessionID, &c2); err != nil {
		t.Fatalf("cat by id: %v", err)
	}
	if c1.String() != c2.String() {
		t.Fatalf("cat key/id differ: %q vs %q", c1.String(), c2.String())
	}
}

// TestSceneMeta_ExposesBothIDs ensures scene meta surfaces both session_id and
// session_key so a caller can jump to ls rmb://sessions/<...>/.
func TestSceneMeta_ExposesBothIDs(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	nowMS := time.Now().UTC().UnixMilli()
	sessionID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	if _, err := s.db.Exec(`
		INSERT INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES (?, 'scene-session', '', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	sceneID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	if _, err := s.db.Exec(`
		INSERT INTO scenes (id, session_id, display_name, abstract, body, source_atoms, created_at, updated_at)
		VALUES (?, ?, 'nm', 'ab', 'body', '[]', ?, ?)`, sceneID, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := s.Meta(ctx, "rmb://scenes/"+sceneID, &buf); err != nil {
		t.Fatalf("meta scene: %v", err)
	}
	meta := buf.String()
	for _, want := range []string{
		`"session_id": "` + sessionID + `"`,
		`"session_key": "scene-session"`,
	} {
		if !strings.Contains(meta, want) {
			t.Fatalf("scene meta missing %s:\n%s", want, meta)
		}
	}
}

// TestSessionMeta_ExposesID verifies session meta (via either key or id) also
// returns the sibling identifier.
func TestSessionMeta_ExposesID(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	nowMS := time.Now().UTC().UnixMilli()
	sessionID := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	if _, err := s.db.Exec(`
		INSERT INTO sessions (id, session_key, abstract, created_at, updated_at)
		VALUES (?, 'meta-session', '', ?, ?)`, sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.Meta(ctx, "rmb://sessions/meta-session", &buf); err != nil {
		t.Fatalf("meta session by key: %v", err)
	}
	if !strings.Contains(buf.String(), `"session_id": "`+sessionID+`"`) {
		t.Fatalf("session meta missing id: %s", buf.String())
	}
	buf.Reset()
	if err := s.Meta(ctx, "rmb://sessions/"+sessionID, &buf); err != nil {
		t.Fatalf("meta session by id: %v", err)
	}
	if !strings.Contains(buf.String(), `"session_key": "meta-session"`) {
		t.Fatalf("session meta by id missing key: %s", buf.String())
	}
}
