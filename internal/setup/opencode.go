package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func opencodePaths() (hooksPath, recallPath string) {
	home, _ := os.UserHomeDir()
	configBase := filepath.Join(home, ".config", "opencode")
	return filepath.Join(configBase, "hook", "hooks.yaml"), filepath.Join(configBase, "agents", "rmb-recall.md")
}

func previewOpenCode(def agentDef) (AgentState, error) {
	hooksPath, recallPath := opencodePaths()
	cmd, err := hookCommand(def.HookSource)
	if err != nil {
		return AgentState{}, err
	}

	currentHooks, hooksExists, err := readFile(hooksPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedHooks, _ := mergeOpenCodeHooksYAML(currentHooks, cmd)
	hookConfigured := strings.Contains(currentHooks, "rmb hook-submit") || strings.Contains(currentHooks, "rmb-hook-dual")

	currentRecall, recallExists, err := readFile(recallPath)
	if err != nil {
		return AgentState{}, err
	}
	proposedRecall, _ := mergeRecallMarkdown(currentRecall)
	if strings.TrimSpace(currentRecall) == "" {
		proposedRecall = "# rmb recall\n\n" + proposedRecall
	}

	artifacts := []Artifact{
		artifactFromStrings(
			"hooks",
			"Conversation capture",
			hooksPath,
			"Adds a session-end hook that submits conversation turns to rmbd.",
			currentHooks,
			proposedHooks,
			hooksExists,
			ApplyWrite,
			[]string{"OpenCode hook event names may vary by version — verify session end fires after chats."},
			"markdown",
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
	hooksPath, recallPath := opencodePaths()
	def, _ := agentDefByID(AgentOpenCode)
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
		proposed, _ := mergeOpenCodeHooksYAML(current, cmd)
		return writeFileWithBackup(hooksPath, proposed)
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

func mergeOpenCodeHooksYAML(current, cmd string) (string, bool) {
	if strings.Contains(current, "rmb hook-submit") || strings.Contains(current, "rmb-hook-dual") {
		if strings.Contains(current, cmd) {
			return current, true
		}
		lines := strings.Split(current, "\n")
		for i, line := range lines {
			if strings.Contains(line, "rmb hook-submit") || strings.Contains(line, "rmb-hook-dual") {
				indent := strings.Repeat(" ", len(line)-len(strings.TrimLeft(line, " ")))
				lines[i] = indent + `- bash: "` + cmd + `"`
				return strings.Join(lines, "\n") + "\n", true
			}
		}
	}
	block := `hooks:
  - id: rmb-submit
    event: session.end
    actions:
      - bash: "` + cmd + `"
`
	if strings.TrimSpace(current) == "" {
		return block, true
	}
	if strings.Contains(current, "rmb-submit") {
		return current, true
	}
	return strings.TrimRight(current, "\n") + "\n\n" + block, true
}
