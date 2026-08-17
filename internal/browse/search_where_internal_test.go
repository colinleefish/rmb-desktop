package browse

import (
	"context"
	"database/sql"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/db"
)

func openInternalTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func mustExecInternal(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatalf("exec: %v\nquery: %s", err, query)
	}
}

// TestSessionScopedSearchStaysInSession verifies the parenthesized search
// WHERE keeps the session_id filter intact: a query matching atoms from a
// different session must not leak them into this session's atom/scene lists.
func TestSessionScopedSearchStaysInSession(t *testing.T) {
	database := openInternalTestDB(t)
	svc := NewService(database, nil)
	now := int64(1)
	mustExecInternal(t, database, `INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES
		('s1', 'sess-one', ?, ?),
		('s2', 'sess-two', ?, ?)`, now, now, now, now)
	// Both atoms contain the query word, but they belong to different sessions.
	mustExecInternal(t, database, `INSERT INTO atoms (id, session_id, category, priority, content, source_turn_ids, created_at, updated_at) VALUES
		('a1', 's1', 'entities', 1, 'alpha needle', '[]', ?, ?),
		('a2', 's2', 'entities', 1, 'beta needle', '[]', ?, ?)`, now, now, now, now)

	page, err := svc.listAtoms(context.Background(), ListParams{Limit: 10, Query: "needle"}, "s1")
	if err != nil {
		t.Fatalf("listAtoms: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Content != "alpha needle" {
		t.Fatalf("session-scoped atom search leaked rows: total=%d items=%+v", page.Total, page.Items)
	}
}
