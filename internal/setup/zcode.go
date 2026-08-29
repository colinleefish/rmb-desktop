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
	stopCmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}
	captureCmd, err := hookCaptureCommand(def.ID)
	if err != nil {
		return AgentState{}, err
	}

	currentConfig, configExists, err := readFile(configPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedConfig, _, err := mergeZCodeHooks(currentConfig, stopCmd, captureCmd)
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
		"Two hooks are installed: UserPromptSubmit captures your prompt, Stop uploads the turn.",
		"Restart ZCode (or start a new session) after applying so the hooks take effect.",
	}

	artifacts := []Artifact{
		artifactFromRaw(
			"config",
			"Conversation capture",
			configPath,
			"Adds UserPromptSubmit + Stop hooks to ~/.zcode/cli/config.json and enables hooks.enabled. ZCode's Stop payload carries no user prompt, so the capture hook parks it for pairing.",
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
	stopCmd, err := hookCommand(def.HookSource)
	if err != nil {
		return err
	}
	captureCmd, err := hookCaptureCommand(def.ID)
	if err != nil {
		return err
	}

	switch artifactID {
	case "config":
		current, _, err := readFile(configPath)
		if err != nil {
			return err
		}
		proposed, _, err := mergeZCodeHooks(current, stopCmd, captureCmd)
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

// zcodeHookConfigured reports whether the rmb Stop hook AND the
// UserPromptSubmit capture hook are registered AND hooks.enabled is true.
// ZCode config-file hooks silently never run without the latter, and without
// the capture hook turns upload assistant-only (ZCode's Stop payload carries
// no user prompt), so "configured" must reflect the fully working state.
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
	events, _ := hooks["events"].(map[string]any)
	if events == nil {
		return false
	}
	return zcodeEventHasRMBHook(events, "Stop") && zcodeEventHasRMBHook(events, "UserPromptSubmit")
}

func zcodeEventHasRMBHook(events map[string]any, event string) bool {
	groups, _ := events[event].([]any)
	for _, item := range groups {
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

// mergeZCodeHooks merges the rmb hooks into ~/.zcode/cli/config.json while
// preserving every other setting (mcp, plugins, ...). Two hooks are needed:
//
//   - UserPromptSubmit → rmb hook-capture --agent=zcode, which parks the
//     user's prompt (the Stop payload does not carry it);
//   - Stop → rmb hook-submit --source=zcode, which pairs the parked prompt
//     with the assistant reply and uploads the turn.
//
// It always sets hooks.enabled: true, since ZCode ignores configuration-file
// hooks otherwise. rmb hook commands are normalized to the canonical CLI
// command (no env prefix): the CLI resolves its endpoint dynamically.
func mergeZCodeHooks(current, stopCmd, captureCmd string) (string, bool, error) {
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

	stopConfigured := mergeZCodeEvent(events, "Stop", stopCmd)
	captureConfigured := mergeZCodeEvent(events, "UserPromptSubmit", captureCmd)

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(out), stopConfigured && captureConfigured, nil
}

// mergeZCodeEvent normalizes (or appends) the rmb hook group for one event
// and reports whether the event now carries an rmb hook.
func mergeZCodeEvent(events map[string]any, event, cmd string) bool {
	groups, _ := events[event].([]any)
	var kept []any
	configured := false
	for _, item := range groups {
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
	events[event] = kept
	return configured
}
