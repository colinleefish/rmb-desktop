package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/setup/integrations"
)

func piPaths() (extensionPath, agentsPath, legacyHookScriptPath string) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".pi", "agent")
	return filepath.Join(base, "extensions", "rmb-hook.ts"),
		filepath.Join(base, "AGENTS.md"),
		filepath.Join(base, "hooks", "rmb-stop.sh")
}

func previewPi(def agentDef) (AgentState, error) {
	extensionPath, agentsPath, legacyHookScriptPath := piPaths()

	proposedExtension, err := renderedPiExtension()
	if err != nil {
		return AgentState{}, err
	}

	currentExtension, extensionExists, err := readFile(extensionPath)
	if err != nil {
		return AgentState{}, err
	}
	hookConfigured := integrations.IsRMBPiExtension(currentExtension)

	currentMD, mdExists, err := readFile(agentsPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedMD, _ := mergeRecallMarkdown(currentMD)

	warnings := []string{
		"Restart Pi after applying so the extension reloads from ~/.pi/agent/extensions/.",
	}
	if legacyScript, legacyExists, legacyErr := readFile(legacyHookScriptPath); legacyErr == nil && legacyExists && strings.Contains(legacyScript, "hook-submit") {
		warnings = append(warnings, "An older shell hook exists at ~/.pi/agent/hooks/rmb-stop.sh — you can delete it manually.")
	}

	artifacts := []Artifact{
		artifactReplaceFile(
			"extension",
			"Conversation capture",
			extensionPath,
			"Installs a Pi extension that submits turns to rmbd on agent_settled.",
			currentExtension,
			proposedExtension,
			extensionExists,
			warnings,
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
	extensionPath, agentsPath, _ := piPaths()

	switch artifactID {
	case "extension":
		proposed, err := renderedPiExtension()
		if err != nil {
			return err
		}
		return writeFileWithBackup(extensionPath, proposed)
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

func renderedPiExtension() (string, error) {
	bin, err := RMBPath()
	if err != nil {
		return "", err
	}
	return integrations.RenderPiExtension(bin)
}
