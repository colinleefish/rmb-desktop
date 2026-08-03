package setup

import (
	"os"
	"path/filepath"
)

// AgentID identifies a supported coding agent integration.
type AgentID string

const (
	AgentCursor     AgentID = "cursor"
	AgentClaudeCode AgentID = "claude-code"
	AgentCodex      AgentID = "codex"
	AgentOpenCode   AgentID = "opencode"
	AgentPi         AgentID = "pi"
)

type agentDef struct {
	ID          AgentID
	Name        string
	Label       string
	Description string
	HookSource  string
	DetectPaths []string
}

var agentDefs = []agentDef{
	{
		ID:          AgentCursor,
		Name:        "Cursor",
		Label:       "Cursor",
		Description: "Capture conversations via stop hook and teach the agent to recall with rmb.",
		HookSource:  "cursor",
		DetectPaths: []string{".cursor"},
	},
	{
		ID:          AgentClaudeCode,
		Name:        "Claude Code",
		Label:       "Claude Code",
		Description: "Stop hook in settings.json plus recall block in CLAUDE.md.",
		HookSource:  "cc",
		DetectPaths: []string{".claude"},
	},
	{
		ID:          AgentCodex,
		Name:        "Codex",
		Label:       "Codex",
		Description: "Stop hook in hooks.json plus recall block in AGENTS.md.",
		HookSource:  "codex",
		DetectPaths: []string{".codex"},
	},
	{
		ID:          AgentOpenCode,
		Name:        "OpenCode",
		Label:       "OpenCode",
		Description: "Session hook plus recall instructions for OpenCode.",
		HookSource:  "opencode",
		DetectPaths: []string{".config/opencode", ".local/share/opencode"},
	},
	{
		ID:          AgentPi,
		Name:        "Pi",
		Label:       "Pi",
		Description: "Recall block in AGENTS.md plus hook script for Pi.",
		HookSource:  "pi",
		DetectPaths: []string{".pi"},
	},
}

func parseAgentID(raw string) (AgentID, bool) {
	switch AgentID(raw) {
	case AgentCursor, AgentClaudeCode, AgentCodex, AgentOpenCode, AgentPi:
		return AgentID(raw), true
	case "cc", "claude":
		return AgentClaudeCode, true
	default:
		return "", false
	}
}

func agentDefByID(id AgentID) (agentDef, bool) {
	for _, d := range agentDefs {
		if d.ID == id {
			return d, true
		}
	}
	return agentDef{}, false
}

func homePaths(rel ...string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	out := make([]string, len(rel))
	for i, r := range rel {
		out[i] = filepath.Join(home, r)
	}
	return out
}
