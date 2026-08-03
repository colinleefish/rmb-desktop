package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cursorPaths() (hooksPath string) {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor", "hooks.json")
}

func previewCursor(def agentDef) (AgentState, error) {
	hooksPath := cursorPaths()
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}

	currentHooks, hooksExists, err := readFile(hooksPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedHooks, _, err := mergeCursorHooks(currentHooks, cmd)
	if err != nil {
		return AgentState{}, err
	}
	hookConfigured := cursorHookConfigured(currentHooks)
	proposedHooksPretty, err := prettyJSON(proposedHooks)
	if err != nil {
		return AgentState{}, err
	}
	currentHooksPretty := currentHooks
	if hooksExists && strings.TrimSpace(currentHooks) != "" {
		currentHooksPretty, _ = prettyJSON(currentHooks)
	}

	rulesCurrent := ""
	rulesProposed, _ := mergeRecallMarkdown(rulesCurrent)

	artifacts := []Artifact{
		artifactFromRaw(
			"hooks",
			"Conversation capture",
			hooksPath,
			"Runs rmb hook-submit when an agent conversation ends.",
			currentHooks,
			proposedHooks,
			currentHooksPretty,
			proposedHooksPretty,
			hooksExists,
			ApplyWrite,
			nil,
			"json",
		),
		artifactFromStrings(
			"user_rules",
			"Recall instructions",
			"Cursor Settings → Rules",
			"Teaches the agent to run rmb search and cat at session start.",
			rulesCurrent,
			rulesProposed,
			false,
			ApplyCopyOnly,
			[]string{"Cursor stores user rules internally — preview only, copy to paste into Settings → Rules."},
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
		RecallStatus: recallStatus(false),
		Artifacts:    artifacts,
	}, nil
}

func applyCursor(artifactID string) error {
	hooksPath := cursorPaths()
	def, _ := agentDefByID(AgentCursor)
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return err
	}

	switch artifactID {
	case "hooks":
		current, _, err := readFile(hooksPath)
		if err != nil {
			return err
		}
		proposed, _, err := mergeCursorHooks(current, cmd)
		if err != nil {
			return err
		}
		pretty, err := prettyJSON(proposed)
		if err != nil {
			return err
		}
		return writeFileWithBackup(hooksPath, pretty)
	case "user_rules":
		return applyErrorCopyOnly(artifactID)
	default:
		return fmt.Errorf("unknown artifact %q", artifactID)
	}
}

func mergeCursorHooks(current, cmd string) (string, bool, error) {
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
	if _, ok := root["version"]; !ok {
		root["version"] = 1
	}

	stop, _ := hooks["stop"].([]any)
	var kept []any
	configured := false
	for _, item := range stop {
		m, ok := item.(map[string]any)
		if !ok {
			kept = append(kept, item)
			continue
		}
		command, _ := m["command"].(string)
		if isRMBHookCommand(command) {
			configured = true
			if _, ok := m["timeout"]; !ok {
				m["timeout"] = 15
			}
			kept = append(kept, m)
			continue
		}
		kept = append(kept, item)
	}
	if !configured {
		kept = append(kept, map[string]any{
			"command": cmd,
			"timeout": 15,
		})
		configured = true
	}
	hooks["stop"] = kept

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", false, err
	}
	return string(out), configured, nil
}

func cursorHookConfigured(current string) bool {
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
	stop, _ := hooks["stop"].([]any)
	for _, item := range stop {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		command, _ := m["command"].(string)
		if isRMBHookCommand(command) {
			return true
		}
	}
	return false
}
