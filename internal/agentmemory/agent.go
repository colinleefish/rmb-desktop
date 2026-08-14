package agentmemory

import (
	_ "embed"
	"strings"
)

//go:embed agent_guide.md
var agentGuideBody string

const agentAbstract = "Agent recall guide"

// AgentGuideBody returns the bundled rmb://agent guide content.
//
// The agent guide is build-time documentation, not a distilled memory. It is
// served directly by the inspect layer for rmb://agent (cat/meta/tree) and is
// never stored in the memories table. This keeps the content deterministic per
// build and avoids a duplicated source of truth that can drift.
func AgentGuideBody() string {
	return strings.TrimSpace(agentGuideBody)
}

// AgentGuideAbstract returns the short abstract for the rmb://agent guide.
func AgentGuideAbstract() string {
	return agentAbstract
}
