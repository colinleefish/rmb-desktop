package setup

import (
	"strings"
	"testing"
)

const zcodeConfigSample = `{
  "mcp": {
    "servers": {
      "example": {"command": "example-mcp"}
    }
  },
  "plugins": {
    "enabled": true
  },
  "hooks": {
    "enabled": true,
    "events": {
      "Stop": [
        {
          "hooks": [
            {
              "type": "command",
              "command": "env RMB_URL=https://rmb.colinleefish.com /Users/liguanghui/.rmb/bin/rmb hook-submit --source=zcode",
              "timeout": 15
            }
          ]
        }
      ],
      "UserPromptSubmit": [
        {
          "hooks": [
            {
              "type": "command",
              "command": "/Users/liguanghui/.rmb/bin/rmb hook-capture --agent=zcode",
              "timeout": 15
            }
          ]
        }
      ]
    }
  }
}`

// zcodeStopOnlySample is what F07 (pre-bugfix) installs wrote: Stop hook only.
const zcodeStopOnlySample = `{
  "hooks": {
    "enabled": true,
    "events": {
      "Stop": [
        {
          "hooks": [
            {
              "type": "command",
              "command": "/Users/liguanghui/.rmb/bin/rmb hook-submit --source=zcode",
              "timeout": 15
            }
          ]
        }
      ]
    }
  }
}`

func TestMergeZCodeHooksInstallsBothHooks(t *testing.T) {
	proposed, configured, err := mergeZCodeHooks(`{}`, "/bin/rmb hook-submit --source=zcode", "/bin/rmb hook-capture --agent=zcode")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, `"enabled": true`) {
		t.Fatalf("hooks.enabled must be true (ZCode ignores hooks otherwise): %s", proposed)
	}
	if !strings.Contains(proposed, `"Stop"`) {
		t.Fatalf("missing Stop hook: %s", proposed)
	}
	if !strings.Contains(proposed, `"UserPromptSubmit"`) {
		t.Fatalf("missing UserPromptSubmit capture hook: %s", proposed)
	}
	if !strings.Contains(proposed, "hook-submit --source=zcode") {
		t.Fatalf("missing submit command: %s", proposed)
	}
	if !strings.Contains(proposed, "hook-capture --agent=zcode") {
		t.Fatalf("missing capture command: %s", proposed)
	}
}

func TestMergeZCodeHooksPreservesSettings(t *testing.T) {
	proposed, configured, err := mergeZCodeHooks(zcodeConfigSample, "/bin/rmb hook-submit --source=zcode", "/bin/rmb hook-capture --agent=zcode")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, `"example-mcp"`) {
		t.Fatalf("lost mcp servers: %s", proposed)
	}
	if !strings.Contains(proposed, `"plugins"`) {
		t.Fatalf("lost plugins config: %s", proposed)
	}
	// Existing RMB commands must be normalized to the canonical dynamic command.
	if strings.Contains(proposed, "RMB_URL") {
		t.Fatalf("RMB_URL env prefix must be stripped: %s", proposed)
	}
	if strings.Count(proposed, "hook-submit --source=zcode") != 1 {
		t.Fatalf("expected exactly one submit hook: %s", proposed)
	}
	if strings.Count(proposed, "hook-capture --agent=zcode") != 1 {
		t.Fatalf("expected exactly one capture hook: %s", proposed)
	}
}

// Regression: F07 installs only carried the Stop hook; re-applying must add
// the UserPromptSubmit capture hook (ZCode's Stop payload has no user prompt).
func TestMergeZCodeHooksMigratesStopOnlyInstall(t *testing.T) {
	proposed, configured, err := mergeZCodeHooks(zcodeStopOnlySample, "/bin/rmb hook-submit --source=zcode", "/bin/rmb hook-capture --agent=zcode")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured after migration")
	}
	if !strings.Contains(proposed, `"UserPromptSubmit"`) {
		t.Fatalf("capture hook not added: %s", proposed)
	}
	if strings.Count(proposed, "hook-submit --source=zcode") != 1 {
		t.Fatalf("expected exactly one submit hook: %s", proposed)
	}
	if zcodeHookConfigured(zcodeStopOnlySample) {
		t.Fatal("stop-only install must report unconfigured (user prompts would be lost)")
	}
}

func TestMergeZCodeHooksIdempotent(t *testing.T) {
	first, _, err := mergeZCodeHooks("", "/bin/rmb hook-submit --source=zcode", "/bin/rmb hook-capture --agent=zcode")
	if err != nil {
		t.Fatal(err)
	}
	second, configured, err := mergeZCodeHooks(first, "/bin/rmb hook-submit --source=zcode", "/bin/rmb hook-capture --agent=zcode")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured on second pass")
	}
	if strings.Count(second, "hook-submit --source=zcode") != 1 {
		t.Fatalf("expected one submit entry, got: %s", second)
	}
	if strings.Count(second, "hook-capture --agent=zcode") != 1 {
		t.Fatalf("expected one capture entry, got: %s", second)
	}
	if changeTypeForJSON(first, second, true) != ChangeUnchanged {
		t.Fatalf("expected unchanged on second pass: %s", second)
	}
}

func TestZCodeHookConfigured(t *testing.T) {
	if !zcodeHookConfigured(zcodeConfigSample) {
		t.Fatal("expected configured for sample config")
	}
	if zcodeHookConfigured(`{"plugins":{"enabled":true}}`) {
		t.Fatal("expected not configured for empty config")
	}
	// Hooks registered but hooks.enabled is false (or absent) must report
	// unconfigured — ZCode silently drops configuration-file hooks otherwise.
	disabled := strings.Replace(zcodeConfigSample, `"hooks": {
    "enabled": true,`, `"hooks": {
    "enabled": false,`, 1)
	if zcodeHookConfigured(disabled) {
		t.Fatal("expected not configured when hooks.enabled is false")
	}
}

func TestPreviewZCodeDetected(t *testing.T) {
	def, _ := agentDefByID(AgentZCode)
	state, err := previewZCode(def)
	if err != nil {
		t.Fatal(err)
	}
	if state.ID != "zcode" {
		t.Fatalf("id = %q", state.ID)
	}
	if len(state.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(state.Artifacts))
	}
	if state.Artifacts[0].ID != "config" || state.Artifacts[0].Language != "json" {
		t.Fatalf("artifact 0 wrong: %+v", state.Artifacts[0])
	}
	if state.Artifacts[1].ID != "agents_md" || state.Artifacts[1].Language != "markdown" {
		t.Fatalf("artifact 1 wrong: %+v", state.Artifacts[1])
	}
}
