package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func codexPaths() (hooksPath, agentsPath string) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".codex")
	return filepath.Join(base, "hooks.json"), filepath.Join(base, "AGENTS.md")
}

func previewCodex(def agentDef) (AgentState, error) {
	hooksPath, agentsPath := codexPaths()
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}

	currentHooks, hooksExists, err := readFile(hooksPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedHooks, _, err := mergeCodexHooks(currentHooks, cmd)
	if err != nil {
		return AgentState{}, err
	}
	hookConfigured := codexHookConfigured(currentHooks)
	proposedPretty, err := prettyJSON(proposedHooks)
	if err != nil {
		return AgentState{}, err
	}
	currentPretty := currentHooks
	if hooksExists && strings.TrimSpace(currentHooks) != "" {
		currentPretty, _ = prettyJSON(currentHooks)
	}

	currentMD, mdExists, err := readFile(agentsPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedMD, _ := mergeRecallMarkdown(currentMD)

	artifacts := []Artifact{
		artifactFromStrings(
			"hooks",
			"Conversation capture",
			hooksPath,
			"Adds a Stop hook that submits conversation turns to rmbd.",
			currentPretty,
			proposedPretty,
			hooksExists,
			ApplyWrite,
			nil,
			"json",
		),
		artifactFromStrings(
			"agents_md",
			"Recall instructions",
			agentsPath,
			"User-level instructions read by Codex at session start.",
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

func codexHookConfigured(current string) bool {
	if strings.TrimSpace(current) == "" {
		return false
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(current), &root); err != nil {
		return false
	}
	for _, item := range codexStopGroups(root) {
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

func applyCodex(artifactID string) error {
	hooksPath, agentsPath := codexPaths()
	def, _ := agentDefByID(AgentCodex)
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
		proposed, _, err := mergeCodexHooks(current, cmd)
		if err != nil {
			return err
		}
		pretty, err := prettyJSON(proposed)
		if err != nil {
			return err
		}
		return writeFileWithBackup(hooksPath, pretty)
	case "agents_md":
		current, _, err := readFile(agentsPath)
		if err != nil {
			return err
		}
		proposed, _ := mergeRecallMarkdown(current)
		return writeFileWithBackup(agentsPath, proposed)
	default:
		return fmt.Errorf("unknown artifact %q", artifactID)
	}
}

func codexStopGroups(root map[string]any) []any {
	if hooks, ok := root["hooks"].(map[string]any); ok {
		if stop, ok := hooks["Stop"].([]any); ok {
			return stop
		}
	}
	if stop, ok := root["Stop"].([]any); ok {
		return stop
	}
	return nil
}

func mergeCodexHooks(current, cmd string) (string, bool, error) {
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

	// Migrate legacy top-level Stop into hooks.Stop.
	if legacy, ok := root["Stop"].([]any); ok && len(legacy) > 0 {
		existing, _ := hooks["Stop"].([]any)
		if len(existing) == 0 {
			hooks["Stop"] = legacy
		}
		delete(root, "Stop")
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
