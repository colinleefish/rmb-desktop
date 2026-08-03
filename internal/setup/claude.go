package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func claudePaths() (settingsPath, claudeMDPath, localSettingsPath string) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".claude")
	return filepath.Join(base, "settings.json"), filepath.Join(base, "CLAUDE.md"), filepath.Join(base, "settings.local.json")
}

func previewClaude(def agentDef) (AgentState, error) {
	settingsPath, claudeMDPath, localSettingsPath := claudePaths()
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}

	currentSettings, settingsExists, err := readFile(settingsPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedSettings, _, err := mergeClaudeSettings(currentSettings, cmd)
	if err != nil {
		return AgentState{}, err
	}
	hookConfigured := claudeHookConfigured(currentSettings)
	proposedPretty, err := prettyJSON(proposedSettings)
	if err != nil {
		return AgentState{}, err
	}
	currentPretty := currentSettings
	displayProposed := proposedSettings
	if settingsExists && strings.TrimSpace(currentSettings) != "" {
		currentPretty = redactSettingsJSON(currentSettings)
		displayProposed = redactSettingsJSON(proposedSettings)
		if p, err := prettyJSON(currentPretty); err == nil {
			currentPretty = p
		}
		if p, err := prettyJSON(displayProposed); err == nil {
			proposedPretty = p
		}
	}

	currentMD, mdExists, err := readFile(claudeMDPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedMD, _ := mergeRecallMarkdown(currentMD)

	warnings := []string{}
	if pathExists(localSettingsPath) {
		warnings = append(warnings, "env values are redacted in preview. Only the Stop hook entry will change.")
		warnings = append(warnings, "~/.claude/settings.local.json exists — this tool only modifies user-level settings.json.")
	} else if settingsExists {
		warnings = append(warnings, "env values are redacted in preview. Only the Stop hook entry will change.")
	}

	artifacts := []Artifact{
		artifactFromRaw(
			"settings",
			"Conversation capture",
			settingsPath,
			"Adds a Stop hook that submits conversation turns to rmbd.",
			currentSettings,
			proposedSettings,
			currentPretty,
			proposedPretty,
			settingsExists,
			ApplyWrite,
			warnings,
			"json",
		),
		artifactFromStrings(
			"claude_md",
			"Recall instructions",
			claudeMDPath,
			"User-level instructions read by Claude Code at session start.",
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

func claudeHookConfigured(current string) bool {
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
	stop, _ := hooks["Stop"].([]any)
	for _, item := range stop {
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

func applyClaude(artifactID string) error {
	settingsPath, claudeMDPath, _ := claudePaths()
	def, _ := agentDefByID(AgentClaudeCode)
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
		proposed, _, err := mergeClaudeSettings(current, cmd)
		if err != nil {
			return err
		}
		pretty, err := prettyJSON(proposed)
		if err != nil {
			return err
		}
		return writeFileWithBackup(settingsPath, pretty)
	case "claude_md":
		current, _, err := readFile(claudeMDPath)
		if err != nil {
			return err
		}
		proposed, _ := mergeRecallMarkdown(current)
		return writeFileWithBackup(claudeMDPath, proposed)
	default:
		return fmt.Errorf("unknown artifact %q", artifactID)
	}
}

func mergeClaudeSettings(current, cmd string) (string, bool, error) {
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
