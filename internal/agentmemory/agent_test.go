package agentmemory

import (
	"strings"
	"testing"
)

// TestGuideTeachesMergedRetrieval asserts the embedded agent guide documents
// the retrieval behavior merged in P0.1–P0.3 so a fresh agent reading only
// rmb://agent can find recent items and drill into the evidence behind a
// memory. Keep this in sync with agent_guide.md.
func TestGuideTeachesMergedRetrieval(t *testing.T) {
	body := AgentGuideBody()

	required := []string{
		"--since",              // recency routing (P0.3, PR #36)
		"--until",              // time window (P0.3, PR #36)
		"--scope=scene",        // explicit scene drill-down (P0.2, PR #39)
		"scene depth",          // linked-scene annotation (P0.2, PR #39)
		"source_scene_uris",    // where to find a memory's evidence scenes
		"v=",                   // version-count trust signal (P0.2, PR #39)
		"rmb://events/2026-06", // ls prefix browse (P0.1, PR #37)
		"--limit",              // ls pagination (P0.1, PR #37)
		"--offset",             // ls pagination (P0.1, PR #37)
	}

	for _, s := range required {
		if !strings.Contains(body, s) {
			t.Errorf("embedded agent guide missing %q", s)
		}
	}

	// Scenes must NOT be a default scope anymore (P0.2) — the guide must not
	// claim scenes are included by default.
	if strings.Contains(body, "default scope includes memory, scene, and skills") {
		t.Error("guide still claims scenes are in the default scope (P0.2)")
	}
}
