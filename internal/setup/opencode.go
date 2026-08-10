package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/setup/integrations"
)

func opencodePaths() (pluginPath, recallPath, legacyHooksPath string) {
	home, _ := os.UserHomeDir()
	configBase := filepath.Join(home, ".config", "opencode")
	return filepath.Join(configBase, "plugin", "rmb-hook.ts"),
		filepath.Join(configBase, "agents", "rmb-recall.md"),
		filepath.Join(configBase, "hook", "hooks.yaml")
}

func previewOpenCode(def agentDef) (AgentState, error) {
	pluginPath, recallPath, legacyHooksPath := opencodePaths()

	proposedPlugin, err := renderedOpenCodePlugin()
	if err != nil {
		return AgentState{}, err
	}

	currentPlugin, pluginExists, err := readFile(pluginPath)
	if err != nil {
		return AgentState{}, err
	}
	hookConfigured := integrations.IsRMBOpenCodePlugin(currentPlugin)

	currentRecall, recallExists, err := readFile(recallPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedRecall, _ := mergeRecallMarkdown(currentRecall)
	if strings.TrimSpace(currentRecall) == "" {
		proposedRecall = "# rmb recall\n\n" + proposedRecall
	}

	warnings := []string{
		"Restart OpenCode after applying so the plugin reloads from ~/.config/opencode/plugin/.",
	}
	if legacyHooks, legacyExists, legacyErr := readFile(legacyHooksPath); legacyErr == nil && legacyExists && strings.Contains(legacyHooks, "rmb hook-submit") {
		warnings = append(warnings, "An older shell-hook config exists at ~/.config/opencode/hook/hooks.yaml — you can delete it manually.")
	}

	artifacts := []Artifact{
		artifactReplaceFile(
			"plugin",
			"Conversation capture",
			pluginPath,
			"Installs an in-process TypeScript plugin that submits turns to rmbd when OpenCode goes idle.",
			currentPlugin,
			proposedPlugin,
			pluginExists,
			warnings,
		),
		artifactFromStrings(
			"recall_md",
			"Recall instructions",
			recallPath,
			"Agent markdown loaded by OpenCode for rmb recall behavior.",
			currentRecall,
			proposedRecall,
			recallExists,
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
		RecallStatus: recallStatus(hasRecallBlock(currentRecall)),
		Artifacts:    artifacts,
	}, nil
}

func applyOpenCode(artifactID string) error {
	pluginPath, recallPath, _ := opencodePaths()

	switch artifactID {
	case "plugin":
		proposed, err := renderedOpenCodePlugin()
		if err != nil {
			return err
		}
		return writeFileWithBackup(pluginPath, proposed)
	case "recall_md":
		current, _, err := readFile(recallPath)
		if err != nil {
			return err
		}
		proposed, _ := mergeRecallMarkdown(current)
		if strings.TrimSpace(current) == "" {
			proposed = "# rmb recall\n\n" + proposed
		}
		return writeFileWithBackup(recallPath, proposed)
	default:
		return fmt.Errorf("unknown artifact %q", artifactID)
	}
}

func renderedOpenCodePlugin() (string, error) {
	bin, err := RMBPath()
	if err != nil {
		return "", err
	}
	return integrations.RenderOpenCodePlugin(bin)
}
