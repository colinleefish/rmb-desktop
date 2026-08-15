package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func workbuddyPaths() (settingsPath, memoryPath string) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".workbuddy")
	return filepath.Join(base, "settings.json"), filepath.Join(base, "MEMORY.md")
}

func previewWorkBuddy(def agentDef) (AgentState, error) {
	settingsPath, memoryPath := workbuddyPaths()
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}

	currentSettings, settingsExists, err := readFile(settingsPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedSettings, _, err := mergeWorkBuddyHooks(currentSettings, cmd)
	if err != nil {
		return AgentState{}, err
	}
	hookConfigured := workbuddyHookConfigured(currentSettings)
	proposedPretty, err := prettyJSON(proposedSettings)
	if err != nil {
		return AgentState{}, err
	}
	currentPretty := currentSettings
	if settingsExists && strings.TrimSpace(currentSettings) != "" {
		currentPretty, _ = prettyJSON(currentSettings)
	}

	currentMD, mdExists, err := readFile(memoryPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedMD, _ := mergeRecallMarkdown(currentMD)

	warnings := []string{
		"Restart WorkBuddy (or start a new session) after applying so the hook takes effect.",
	}

	artifacts := []Artifact{
		artifactFromStrings(
			"settings",
			"Conversation capture",
			settingsPath,
			"Adds a Stop hook to ~/.workbuddy/settings.json that submits turns to rmbd.",
			currentPretty,
			proposedPretty,
			settingsExists,
			ApplyWrite,
			nil,
			"json",
		),
		artifactFromStrings(
			"memory_md",
			"Recall instructions",
			memoryPath,
			"User-level memory read by WorkBuddy at session start.",
			currentMD,
			proposedMD,
			mdExists,
			ApplyWrite,
			warnings,
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

func applyWorkBuddy(artifactID string) error {
	settingsPath, memoryPath := workbuddyPaths()
	def, _ := agentDefByID(AgentWorkBuddy)
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return err
	}

	switch artifactID {
	case "settings":
		current, _, err := readFile(settingsPath)
		if err != nil {
			return err
		}
		proposed, _, err := mergeWorkBuddyHooks(current, cmd)
		if err != nil {
			return err
		}
		pretty, err := prettyJSON(proposed)
		if err != nil {
			return err
		}
		return writeFileWithBackup(settingsPath, pretty)
	case "memory_md":
		current, _, err := readFile(memoryPath)
		if err != nil {
			return err
		}
		proposed, _ := mergeRecallMarkdown(current)
		return writeFileWithBackup(memoryPath, proposed)
	default:
		return fmt.Errorf("unknown artifact %q", artifactID)
	}
}

// workbuddyStopGroups returns the hooks.Stop list from the WorkBuddy
// settings.json shape: hooks.Stop[].hooks[].
func workbuddyStopGroups(root map[string]any) []any {
	if hooks, ok := root["hooks"].(map[string]any); ok {
		if stop, ok := hooks["Stop"].([]any); ok {
			return stop
		}
	}
	return nil
}

func workbuddyHookConfigured(current string) bool {
	if strings.TrimSpace(current) == "" {
		return false
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(current), &root); err != nil {
		return false
	}
	for _, item := range workbuddyStopGroups(root) {
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

// mergeWorkBuddyHooks merges the rmb Stop hook into a WorkBuddy settings.json
// while preserving every other setting (enabledPlugins, sandbox, claw, ...).
// rmb hook commands are normalized to the canonical CLI command (no env prefix
// such as RMB_URL=...): the CLI resolves its endpoint dynamically (defaults to
// the local rmbd on 127.0.0.1:19019, overridable via config or --url).
func mergeWorkBuddyHooks(current, cmd string) (string, bool, error) {
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

	stop, _ := hooks["Stop"].([]any)
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
				// Normalize to the canonical dynamic command: strip any env prefix
				// (e.g. RMB_URL=https://...) so the hook never hardcodes an endpoint.
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
	hooks["Stop"] = kept

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(out), configured, nil
}
