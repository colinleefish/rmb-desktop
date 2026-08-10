package integrations

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed opencode/rmb-hook.ts
var openCodePluginTemplate string

//go:embed pi/rmb-hook.ts
var piExtensionTemplate string

const rmbHookBinPlaceholder = "__RMB_HOOK_BIN_JSON__"

// RenderOpenCodePlugin returns the OpenCode plugin source with the rmb binary path injected.
func RenderOpenCodePlugin(rmbBinPath string) (string, error) {
	return renderIntegrationTemplate(openCodePluginTemplate, "opencode plugin", rmbBinPath)
}

// RenderPiExtension returns the Pi extension source with the rmb binary path injected.
func RenderPiExtension(rmbBinPath string) (string, error) {
	return renderIntegrationTemplate(piExtensionTemplate, "pi extension", rmbBinPath)
}

func renderIntegrationTemplate(template, label, rmbBinPath string) (string, error) {
	rmbBinPath = strings.TrimSpace(rmbBinPath)
	if rmbBinPath == "" {
		return "", fmt.Errorf("rmb binary path is required")
	}
	quoted, err := json.Marshal(rmbBinPath)
	if err != nil {
		return "", fmt.Errorf("encode rmb binary path: %w", err)
	}
	if !strings.Contains(template, rmbHookBinPlaceholder) {
		return "", fmt.Errorf("%s template missing %q placeholder", label, rmbHookBinPlaceholder)
	}
	return strings.Replace(template, rmbHookBinPlaceholder, string(quoted), 1), nil
}

// IsRMBOpenCodePlugin reports whether content looks like the rmb OpenCode plugin.
func IsRMBOpenCodePlugin(content string) bool {
	c := strings.ToLower(content)
	return strings.Contains(c, "rmb-hook") &&
		strings.Contains(c, "hook-submit") &&
		strings.Contains(c, "--source=opencode")
}

// IsRMBPiExtension reports whether content looks like the rmb Pi extension.
func IsRMBPiExtension(content string) bool {
	c := strings.ToLower(content)
	return strings.Contains(c, "rmb-hook") &&
		strings.Contains(c, "hook-submit") &&
		strings.Contains(c, "--source=pi") &&
		strings.Contains(c, "agent_settled")
}

// OpenCodePluginTemplate returns the unrendered embedded template (for tests).
func OpenCodePluginTemplate() string {
	return openCodePluginTemplate
}

// PiExtensionTemplate returns the unrendered embedded template (for tests).
func PiExtensionTemplate() string {
	return piExtensionTemplate
}
