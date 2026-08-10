package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/agentmemory"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

func TestMigrationSeedsAgentMemory(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	var count int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM memories
		WHERE uri = 'rmb://agent' AND category = 'agent' AND superseded_at IS NULL`,
	).Scan(&count); err != nil {
		t.Fatalf("query agent memory: %v", err)
	}
	if count != 1 {
		t.Fatalf("agent memory count = %d, want 1", count)
	}

	var body string
	if err := database.QueryRow(`
		SELECT body FROM memories
		WHERE uri = 'rmb://agent' AND superseded_at IS NULL`,
	).Scan(&body); err != nil {
		t.Fatalf("query agent body: %v", err)
	}
	for _, want := range []string{"Memory pyramid", "CLI rules", "rmb://profile"} {
		if !strings.Contains(body, want) {
			t.Fatalf("agent body missing %q", want)
		}
	}

	if err := agentmemory.UpsertAgentGuide(context.Background(), database); err != nil {
		t.Fatalf("UpsertAgentGuide: %v", err)
	}
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM memories
		WHERE uri = 'rmb://agent' AND superseded_at IS NULL`,
	).Scan(&count); err != nil {
		t.Fatalf("count active agent rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("active agent rows = %d, want 1", count)
	}
}

func TestUpsertAgentGuide_createsAndUpdates(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx := context.Background()
	body := readActiveAgentBody(t, database)
	if body != agentmemory.AgentGuideBody() {
		t.Fatal("initial agent guide body mismatch")
	}

	updatedBody := body + "\n\nUpdated."
	if err := agentmemory.UpsertAgentGuideBody(ctx, database, updatedBody); err != nil {
		t.Fatalf("upsert updated body: %v", err)
	}
	if got := readActiveAgentBody(t, database); got != updatedBody {
		t.Fatalf("updated body = %q", got)
	}

	var version int
	if err := database.QueryRow(`
		SELECT version FROM memories
		WHERE uri = ? AND superseded_at IS NULL`, uri.BuildAgent(),
	).Scan(&version); err != nil {
		t.Fatalf("query version: %v", err)
	}
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}

	if err := agentmemory.UpsertAgentGuide(ctx, database); err != nil {
		t.Fatalf("UpsertAgentGuide restore bundled body: %v", err)
	}
	if got := readActiveAgentBody(t, database); got != agentmemory.AgentGuideBody() {
		t.Fatal("bundled body restore failed")
	}
}

func readActiveAgentBody(t *testing.T, database *sql.DB) string {
	t.Helper()
	var body string
	if err := database.QueryRow(`
		SELECT COALESCE(body, '') FROM memories
		WHERE uri = ? AND superseded_at IS NULL`, uri.BuildAgent(),
	).Scan(&body); err != nil {
		t.Fatalf("read agent body: %v", err)
	}
	return body
}
