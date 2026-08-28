package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ZCode's hook config lives under a different shape than Claude/Codex/Cursor:
// configuration-file hooks nest under hooks.events.<Event> (not hooks.<Event>
// directly), and they are disabled by default — hooks.enabled must be true
// or the hook runner never starts, regardless of what's registered.
// See ~/.zcode/cli/plugins/cache/.../zcode-guide skills (diagnosing-hooks,
// zcode-configuration-guide) for the authoritative schema.
func zcodePaths() (configPath, agentsMDPath string) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".zcode")
	return filepath.Join(base, "cli", "config.json"), filepath.Join(base, "AGENTS.md")
}

func previewZCode(def agentDef) (AgentState, error) {
	configPath, agentsMDPath := zcodePaths()
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}

	currentConfig, configExists, err := readFile(configPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedConfig, _, err := mergeZCodeHooks(currentConfig, cmd)
	if err != nil {
		return AgentState{}, err
	}
	hookConfigured := zcodeHookConfigured(currentConfig)
	proposedPretty, err := prettyJSON(proposedConfig)
	if err != nil {
		return AgentState{}, err
	}
	currentPretty := currentConfig
	if configExists && strings.TrimSpace(currentConfig) != "" {
		currentPretty, _ = prettyJSON(currentConfig)
	}

	currentMD, mdExists, err := readFile(agentsMDPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedMD, _ := mergeRecallMarkdown(currentMD)

	warnings := []string{
		"ZCode disables configuration-file hooks unless hooks.enabled is true — this sets it.",
		"Restart ZCode (or start a new session) after applying so the hook takes effect.",
	}

	artifacts := []Artifact{
		artifactFromRaw(
			"config",
			"Conversation capture",
			configPath,
			"Adds a Stop hook to ~/.zcode/cli/config.json (hooks.events.Stop) and enables hooks.enabled.",
			currentConfig,
			proposedConfig,
			currentPretty,
			proposedPretty,
			configExists,
			ApplyWrite,
			warnings,
			"json",
		),
		artifactFromStrings(
			"agents_md",
			"Recall instructions",
			agentsMDPath,
			"User-level instructions read by ZCode at session start.",
			currentMD,
			proposedMD,
			mdExists,
			ApplyWrite,
			nil,
			"markdown",
		),
	}

	detected := detectAgent(def)
	return AgentState{
		ID:           string(def.ID),
		Name:         def.Label,
		Description:  def.Description,
		Detected:     detected,
		HookStatus:   hookStatus(hookConfigured),
		RecallStatus: recallStatus(hasRecallBlock(currentMD)),
		Artifacts:    artifacts,
	}, nil
}

func applyZCode(artifactID string) error {
	configPath, agentsMDPath := zcodePaths()
	def, _ := agentDefByID(AgentZCode)
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return err
	}

	switch artifactID {
	case "config":
		current, _, err := readFile(configPath)
		if err != nil {
			return err
		}
		proposed, _, err := mergeZCodeHooks(current, cmd)
		if err != nil {
			return err
		}
		pretty, err := prettyJSON(proposed)
		if err != nil {
			return err
		}
		return writeFileWithBackup(configPath, pretty)
	case "agents_md":
		current, _, err := readFile(agentsMDPath)
		if err != nil {
			return err
		}
		proposed, _ := mergeRecallMarkdown(current)
		return writeFileWithBackup(agentsMDPath, proposed)
	default:
		return fmt.Errorf("unknown artifact %q", artifactID)
	}
}

// zcodeStopGroups returns the hooks.events.Stop list from ~/.zcode/cli/config.json.
func zcodeStopGroups(root map[string]any) []any {
	if hooks, ok := root["hooks"].(map[string]any); ok {
		if events, ok := hooks["events"].(map[string]any); ok {
			if stop, ok := events["Stop"].([]any); ok {
				return stop
			}
		}
	}
	return nil
}

// zcodeHookConfigured reports whether the rmb Stop hook is registered AND
// hooks.enabled is true. ZCode config-file hooks silently never run without
// the latter, so "configured" must reflect the runnable state, not just
// presence in events.Stop.
func zcodeHookConfigured(current string) bool {
	if strings.TrimSpace(current) == "" {
		return false
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(current), &root); err != nil {
		return false
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return false
	}
	enabled, _ := hooks["enabled"].(bool)
	if !enabled {
		return false
	}
	for _, item := range zcodeStopGroups(root) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := m["hooks"].([]any)
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			command, _ := hm["command"].(string)
			if isRMBHookCommand(command) {
				return true
			}
		}
	}
	return false
}

// mergeZCodeHooks merges the rmb Stop hook into ~/.zcode/cli/config.json
// while preserving every other setting (mcp, plugins, ...). It always sets
// hooks.enabled: true, since ZCode ignores configuration-file hooks
// otherwise. rmb hook commands are normalized to the canonical CLI command
// (no env prefix): the CLI resolves its endpoint dynamically.
func mergeZCodeHooks(current, cmd string) (string, bool, error) {
	root := map[string]any{}
	if strings.TrimSpace(current) != "" {
		if err := json.Unmarshal([]byte(current), &root); err != nil {
			return "", false, err
		}
	}

	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	hooks["enabled"] = true

	events, _ := hooks["events"].(map[string]any)
	if events == nil {
		events = map[string]any{}
		hooks["events"] = events
	}

	stop, _ := events["Stop"].([]any)
	var kept []any
	configured := false
	for _, item := range stop {
		m, ok := item.(map[string]any)
		if !ok {
			kept = append(kept, item)
			continue
		}
		inner, _ := m["hooks"].([]any)
		var innerKept []any
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				innerKept = append(innerKept, h)
				continue
			}
			command, _ := hm["command"].(string)
			if isRMBHookCommand(command) {
				configured = true
				hm["type"] = "command"
				// Normalize to the canonical dynamic command: strip any env
				// prefix so the hook never hardcodes an endpoint.
				hm["command"] = cmd
				if _, ok := hm["timeout"]; !ok {
					hm["timeout"] = 15
				}
				innerKept = append(innerKept, hm)
				continue
			}
			innerKept = append(innerKept, h)
		}
		if len(innerKept) == 0 {
			innerKept = []any{map[string]any{
				"type":    "command",
				"command": cmd,
				"timeout": 15,
			}}
			configured = true
		}
		m["hooks"] = innerKept
		kept = append(kept, m)
	}
	if !configured {
		kept = append(kept, map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": cmd,
					"timeout": 15,
				},
			},
		})
		configured = true
	}
	events["Stop"] = kept

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(out), configured, nil
}
