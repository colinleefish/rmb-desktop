package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func piPaths() (hookScriptPath, agentsPath string) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".pi", "agent")
	return filepath.Join(base, "hooks", "rmb-stop.sh"), filepath.Join(base, "AGENTS.md")
}

func previewPi(def agentDef) (AgentState, error) {
	hookScriptPath, agentsPath := piPaths()
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}

	currentScript, scriptExists, err := readFile(hookScriptPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedScript, _ := mergePiHookScript(currentScript, cmd)
	hookConfigured := strings.Contains(currentScript, "hook-submit")

	currentMD, mdExists, err := readFile(agentsPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedMD, _ := mergeRecallMarkdown(currentMD)

	artifacts := []Artifact{
		artifactFromStrings(
			"hook_script",
			"Conversation capture",
			hookScriptPath,
			"Shell hook script Pi can run at session end.",
			currentScript,
			proposedScript,
			scriptExists,
			ApplyWrite,
			[]string{"Enable this hook in Pi agent settings if hooks are not auto-discovered."},
			"markdown",
		),
		artifactFromStrings(
			"agents_md",
			"Recall instructions",
			agentsPath,
			"User-level instructions read by Pi at session start.",
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

func applyPi(artifactID string) error {
	hookScriptPath, agentsPath := piPaths()
	def, _ := agentDefByID(AgentPi)
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return err
	}

	switch artifactID {
	case "hook_script":
		current, _, err := readFile(hookScriptPath)
		if err != nil {
			return err
		}
		proposed, _ := mergePiHookScript(current, cmd)
		if err := writeFileWithBackup(hookScriptPath, proposed); err != nil {
			return err
		}
		return os.Chmod(hookScriptPath, 0o755)
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

func mergePiHookScript(current, cmd string) (string, bool) {
	script := "#!/usr/bin/env bash\nset -euo pipefail\n" + cmd + "\n"
	if strings.TrimSpace(current) == "" {
		return script, true
	}
	if strings.Contains(current, "rmb hook-submit") || strings.Contains(current, "hook-submit --source=pi") {
		lines := strings.Split(current, "\n")
		for i, line := range lines {
			if strings.Contains(line, "hook-submit") {
				lines[i] = cmd
				return strings.Join(lines, "\n") + "\n", true
			}
		}
	}
	return script, strings.Contains(current, "hook-submit")
}
