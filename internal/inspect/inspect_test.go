package inspect

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/agentmemory"
	"github.com/colinleefish/rmb-desktop/internal/db"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return NewService(d)
}

// TestAgentServedFromBundle verifies rmb://agent is served directly from the
// embedded bundle, with no memories row involved (cat/meta/tree all bypass the
// memories table for the agent scope).
func TestAgentServedFromBundle(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	// cat returns the exact bundled body.
	var buf bytes.Buffer
	if err := s.Cat(ctx, "rmb://agent", &buf); err != nil {
		t.Fatalf("Cat: %v", err)
	}
	if got := buf.String(); got != agentmemory.AgentGuideBody() {
		t.Fatalf("cat agent body mismatch:\ngot:  %q\nwant: %q", got, agentmemory.AgentGuideBody())
	}

	// meta is synthesized from the bundle.
	buf.Reset()
	if err := s.Meta(ctx, "rmb://agent", &buf); err != nil {
		t.Fatalf("Meta: %v", err)
	}
	meta := buf.String()
	for _, want := range []string{`"uri": "rmb://agent"`, `"category": "agent"`, agentmemory.AgentGuideAbstract()} {
		if !strings.Contains(meta, want) {
			t.Fatalf("meta agent missing %q: %s", want, meta)
		}
	}

	// tree always lists the agent URI (it is bundled, so always "present").
	buf.Reset()
	if err := s.Tree(ctx, "rmb://agent", &buf); err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "rmb://agent" {
		t.Fatalf("tree agent = %q, want rmb://agent", got)
	}
}
