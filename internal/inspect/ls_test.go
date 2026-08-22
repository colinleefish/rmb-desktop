package inspect

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// insertMemory adds an active memory row with the given uri and updated_at.
func insertMemory(t *testing.T, s *Service, uri string, updatedAt int64) {
	t.Helper()
	_, err := s.db.Exec(`
		INSERT INTO memories (id, uri, category, version, superseded_at, abstract, body, source_scene_uris, created_at, updated_at)
		VALUES (?, ?, ?, 1, NULL, '', '', '[]', ?, ?)`,
		"mem-"+uri, uri, strings.SplitN(strings.TrimPrefix(uri, "rmb://"), "/", 2)[0], updatedAt, updatedAt)
	if err != nil {
		t.Fatalf("insert memory %s: %v", uri, err)
	}
}

func lsOut(t *testing.T, s *Service, raw string, opts LsOptions) string {
	t.Helper()
	var buf bytes.Buffer
	if err := s.LsWith(context.Background(), raw, opts, &buf); err != nil {
		t.Fatalf("LsWith(%q): %v", raw, err)
	}
	return buf.String()
}

func lsLines(t *testing.T, s *Service, raw string, opts LsOptions) []string {
	t.Helper()
	out := lsOut(t, s, raw, opts)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func TestLsPrefixBrowseByDate(t *testing.T) {
	s := newTestService(t)
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC).UnixMilli()
	insertMemory(t, s, "rmb://events/2026-06-01-summer", now)
	insertMemory(t, s, "rmb://events/2026-06-15-mid", now)
	insertMemory(t, s, "rmb://events/2026-07-01-late", now)
	insertMemory(t, s, "rmb://entities/2026-06-foo", now)

	got := lsLines(t, s, "rmb://events/2026-06", DefaultLsOptions())
	want := []string{"rmb://events/2026-06-01-summer", "rmb://events/2026-06-15-mid"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("prefix browse = %v, want %v", got, want)
	}
}

func TestLsPaginationRoundTrip(t *testing.T) {
	s := newTestService(t)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	for i := 0; i < 5; i++ {
		insertMemory(t, s, fmt.Sprintf("rmb://events/day-%d", i), base+int64(i*24*3600*1000))
	}

	// Default: first 200 (all 5 here), no header.
	got := lsLines(t, s, "rmb://events/", DefaultLsOptions())
	if len(got) != 5 || got[0] != "rmb://events/day-4" || got[4] != "rmb://events/day-0" {
		t.Fatalf("default page wrong: %v", got)
	}

	// Page 1: offset 0, limit 2 → newest two.
	p1 := lsLines(t, s, "rmb://events/", LsOptions{Limit: 2})
	if len(p1) != 2 || p1[0] != "rmb://events/day-4" || p1[1] != "rmb://events/day-3" {
		t.Fatalf("page1 = %v", p1)
	}

	// Page 2: offset 2, limit 2 → next two.
	p2 := lsLines(t, s, "rmb://events/", LsOptions{Limit: 2, Offset: 2})
	if len(p2) != 2 || p2[0] != "rmb://events/day-2" || p2[1] != "rmb://events/day-1" {
		t.Fatalf("page2 = %v", p2)
	}

	// Page 3: offset 4 → last one; offset beyond total → empty.
	p3 := lsLines(t, s, "rmb://events/", LsOptions{Limit: 2, Offset: 4})
	if len(p3) != 1 || p3[0] != "rmb://events/day-0" {
		t.Fatalf("page3 = %v", p3)
	}
	if got := lsLines(t, s, "rmb://events/", LsOptions{Limit: 2, Offset: 9}); len(got) != 0 {
		t.Fatalf("page beyond total = %v, want empty", got)
	}

	// Round-trip: page1 + page2 + page3 covers all rows without overlap.
	seen := map[string]bool{}
	for _, p := range [][]string{p1, p2, p3} {
		for _, line := range p {
			if seen[line] {
				t.Fatalf("duplicate row across pages: %s", line)
			}
			seen[line] = true
		}
	}
	if len(seen) != 5 {
		t.Fatalf("round-trip covered %d rows, want 5", len(seen))
	}
}

func TestLsTimeWindows(t *testing.T) {
	s := newTestService(t)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	day := int64(24 * 3600 * 1000)
	insertMemory(t, s, "rmb://events/aug-01", base)
	insertMemory(t, s, "rmb://events/aug-02", base+day)
	insertMemory(t, s, "rmb://events/aug-03", base+2*day)

	// Since = 2026-08-01 00:00:00.000 UTC → boundary equality keeps aug-01.
	got := lsLines(t, s, "rmb://events/", LsOptions{Since: base, Until: base + day})
	if len(got) != 2 || got[0] != "rmb://events/aug-02" || got[1] != "rmb://events/aug-01" {
		t.Fatalf("boundary window = %v", got)
	}

	// Open-ended until.
	got = lsLines(t, s, "rmb://events/", LsOptions{Until: base + day})
	if len(got) != 2 {
		t.Fatalf("until-only window = %v", got)
	}

	// Open-ended since.
	got = lsLines(t, s, "rmb://events/", LsOptions{Since: base + day})
	if len(got) != 2 || got[0] != "rmb://events/aug-03" {
		t.Fatalf("since-only window = %v", got)
	}

	// Window excluding everything.
	if got := lsLines(t, s, "rmb://events/", LsOptions{Since: base + 3*day}); len(got) != 0 {
		t.Fatalf("empty window = %v", got)
	}
}

func TestLsCount(t *testing.T) {
	s := newTestService(t)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	for i := 0; i < 5; i++ {
		insertMemory(t, s, fmt.Sprintf("rmb://events/day-%d", i), base+int64(i))
	}

	out := lsOut(t, s, "rmb://events/", LsOptions{Count: true, Limit: 2, Offset: 1})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if lines[0] != "showing 2-3 of 5" {
		t.Fatalf("count header = %q, want %q", lines[0], "showing 2-3 of 5")
	}
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got %d lines: %v", len(lines), lines)
	}

	// Count with an empty page.
	out = lsOut(t, s, "rmb://events/", LsOptions{Count: true, Limit: 2, Offset: 99})
	if !strings.HasPrefix(out, "showing 0 of 5\n") {
		t.Fatalf("empty-page count header = %q", out)
	}
}

func TestLsBackwardCompatNoFlags(t *testing.T) {
	s := newTestService(t)
	insertMemory(t, s, "rmb://events/a", 1)
	insertMemory(t, s, "rmb://events/b", 2)

	var buf bytes.Buffer
	if err := s.Ls(context.Background(), "rmb://events/", &buf); err != nil {
		t.Fatalf("Ls: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "rmb://events/b\nrmb://events/a\n") {
		t.Fatalf("plain ls output not backward compatible: %q", buf.String())
	}
	if strings.Contains(buf.String(), "showing") {
		t.Fatalf("plain ls must not print a count header: %q", buf.String())
	}
}

func TestLsScenesAtomsTurnsPrefix(t *testing.T) {
	s := newTestService(t)
	for _, id := range []string{"abc123", "abd456", "zzz9"} {
		_, err := s.db.Exec(`INSERT INTO scenes (id, session_id, abstract, body, source_atoms, created_at, updated_at)
			VALUES (?, 'sess-1', '', '', '[]', 1, 1)`, id)
		if err != nil {
			t.Fatalf("insert scene %s: %v", id, err)
		}
	}
	for _, id := range []string{"atom-x1", "atom-x2", "other"} {
		_, err := s.db.Exec(`INSERT INTO atoms (id, session_id, category, priority, content, source_turn_ids, created_at, updated_at)
			VALUES (?, 'sess-1', 'events', 50, '', '[]', 1, 1)`, id)
		if err != nil {
			t.Fatalf("insert atom %s: %v", id, err)
		}
	}
	for _, id := range []string{"turn-aa1", "turn-aa2", "other-t"} {
		_, err := s.db.Exec(`INSERT INTO session_turns (id, session_id, messages_json, created_at)
			VALUES (?, 'sess-1', '[]', 1)`, id)
		if err != nil {
			t.Fatalf("insert turn %s: %v", id, err)
		}
	}

	scenes := lsLines(t, s, "rmb://scenes/ab", DefaultLsOptions())
	if strings.Join(scenes, "\n") != "rmb://scenes/abc123\nrmb://scenes/abd456" {
		t.Fatalf("scene prefix = %v", scenes)
	}
	atoms := lsLines(t, s, "rmb://atoms/atom-x", DefaultLsOptions())
	if strings.Join(atoms, "\n") != "rmb://atoms/atom-x1\nrmb://atoms/atom-x2" {
		t.Fatalf("atom prefix = %v", atoms)
	}
	turns := lsLines(t, s, "rmb://turns/turn-aa", DefaultLsOptions())
	if strings.Join(turns, "\n") != "rmb://turns/turn-aa1\nrmb://turns/turn-aa2" {
		t.Fatalf("turn prefix = %v", turns)
	}
	// Unknown prefix yields nothing (was previously an exact-match only).
	if got := lsLines(t, s, "rmb://scenes/nope", DefaultLsOptions()); len(got) != 0 {
		t.Fatalf("unknown scene prefix = %v", got)
	}
}

func TestLsSessionsPrefixAndExact(t *testing.T) {
	s := newTestService(t)
	for _, key := range []string{"2026-06-abc", "2026-06-def", "2025-01-old"} {
		_, err := s.db.Exec(`INSERT INTO sessions (id, session_key, abstract, created_at, updated_at)
			VALUES (?, ?, '', 1, 1)`, "sess-"+key, key)
		if err != nil {
			t.Fatalf("insert session %s: %v", key, err)
		}
	}

	// Prefix browse.
	got := lsLines(t, s, "rmb://sessions/2026-06", DefaultLsOptions())
	if len(got) != 2 || !strings.HasPrefix(got[0], "rmb://sessions/2026-06-") {
		t.Fatalf("session prefix = %v", got)
	}

	// Exact session still lists its turns (existing behavior preserved).
	_, err := s.db.Exec(`INSERT INTO session_turns (id, session_id, messages_json, created_at) VALUES ('t1', 'sess-2026-06-abc', '[]', 1)`)
	if err != nil {
		t.Fatalf("insert turn: %v", err)
	}
	exact := lsLines(t, s, "rmb://sessions/2026-06-abc/", DefaultLsOptions())
	if strings.Join(exact, "\n") != "rmb://turns/t1" {
		t.Fatalf("exact session ls = %v", exact)
	}

	// Container listing paginates too.
	got = lsLines(t, s, "rmb://sessions/", LsOptions{Limit: 2})
	if len(got) != 2 {
		t.Fatalf("session page = %v", got)
	}
}

func TestLsTimeFilterOnTurnsUsesCreatedAt(t *testing.T) {
	s := newTestService(t)
	_, err := s.db.Exec(`INSERT INTO sessions (id, session_key, abstract, created_at, updated_at) VALUES ('sess-1', 'key-1', '', 1, 1)`)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	_, err = s.db.Exec(`INSERT INTO session_turns (id, session_id, messages_json, created_at) VALUES ('t1', 'sess-1', '[]', 1000)`)
	if err != nil {
		t.Fatalf("insert turn: %v", err)
	}
	_, err = s.db.Exec(`INSERT INTO session_turns (id, session_id, messages_json, created_at) VALUES ('t2', 'sess-1', '[]', 2000)`)
	if err != nil {
		t.Fatalf("insert turn: %v", err)
	}

	got := lsLines(t, s, "rmb://turns/", LsOptions{Since: 2000})
	if strings.Join(got, "\n") != "rmb://turns/t2" {
		t.Fatalf("turn since filter = %v", got)
	}
}

func TestParseTimeFilter(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		in   string
		want int64
	}{
		{"2026-08-01", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).UnixMilli()},
		{"2026-08-01T00:00:00Z", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC).UnixMilli()},
		{"7d", now.Add(-7 * 24 * time.Hour).UnixMilli()},
		{"30d", now.Add(-30 * 24 * time.Hour).UnixMilli()},
		{"12h", now.Add(-12 * time.Hour).UnixMilli()},
		{"2w", now.Add(-14 * 24 * time.Hour).UnixMilli()},
	}
	for _, c := range cases {
		got, err := ParseTimeFilter(c.in, now)
		if err != nil {
			t.Fatalf("ParseTimeFilter(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("ParseTimeFilter(%q) = %d, want %d", c.in, got, c.want)
		}
	}

	for _, bad := range []string{"", "yesterday", "7x", "2026-13-99"} {
		if _, err := ParseTimeFilter(bad, now); err == nil {
			t.Fatalf("ParseTimeFilter(%q) expected error", bad)
		}
	}
}

func TestLsLimitClamping(t *testing.T) {
	s := newTestService(t)
	insertMemory(t, s, "rmb://events/a", 1)
	opts := LsOptions{Limit: maxLsLimit + 1000}
	if got := lsLines(t, s, "rmb://events/", opts); len(got) != 1 {
		t.Fatalf("clamped page = %v", got)
	}
	if got := lsLines(t, s, "rmb://events/", LsOptions{}); len(got) != 1 {
		t.Fatalf("zero limit defaults to 200, got %v", got)
	}
}
