package db

import (
	"path/filepath"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/agentmemory"
)

// TestMigrationDoesNotSeedAgentMemory asserts the rmb://agent guide is no
// longer stored in the memories table — it is served from the embedded bundle
// by the inspect layer. Both fresh installs and upgraded DBs (after 00008)
// must have zero agent rows.
func TestMigrationDoesNotSeedAgentMemory(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	var count int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM memories WHERE category = 'agent'`).Scan(&count); err != nil {
		t.Fatalf("query agent memory: %v", err)
	}
	if count != 0 {
		t.Fatalf("agent memory count = %d, want 0 (served from embedded bundle, not stored)", count)
	}

	// Sanity: the bundled body is non-empty so the server has something to serve.
	if agentmemory.AgentGuideBody() == "" {
		t.Fatal("AgentGuideBody is empty; embedded agent_guide.md missing or blank")
	}
}
