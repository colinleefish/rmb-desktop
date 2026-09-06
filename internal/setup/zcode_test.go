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
      ]
    }
  }
}`

func TestMergeZCodeHooksPreservesSettings(t *testing.T) {
	proposed, configured, err := mergeZCodeHooks(zcodeConfigSample, "/bin/rmb hook-submit --source=zcode")
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
	// Existing RMB command must be normalized to the canonical dynamic command.
	if strings.Contains(proposed, "RMB_URL") {
		t.Fatalf("RMB_URL env prefix must be stripped: %s", proposed)
	}
	if !strings.Contains(proposed, `"command": "/bin/rmb hook-submit --source=zcode"`) {
		t.Fatalf("expected normalized canonical command: %s", proposed)
	}
}

func TestMergeZCodeHooksAddsStopHookAndEnablesRunner(t *testing.T) {
	current := `{"mcp": {"servers": {}}}`
	proposed, configured, err := mergeZCodeHooks(current, "/bin/rmb hook-submit --source=zcode")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, `"enabled": true`) {
		t.Fatalf("hooks.enabled must be set to true (ZCode ignores hooks otherwise): %s", proposed)
	}
	if !strings.Contains(proposed, `"events"`) || !strings.Contains(proposed, `"Stop"`) {
		t.Fatalf("missing hooks.events.Stop: %s", proposed)
	}
	if !strings.Contains(proposed, "hook-submit --source=zcode") {
		t.Fatalf("missing rmb command: %s", proposed)
	}
}

func TestMergeZCodeHooksIdempotent(t *testing.T) {
	first, _, err := mergeZCodeHooks("", "/bin/rmb hook-submit --source=zcode")
	if err != nil {
		t.Fatal(err)
	}
	second, configured, err := mergeZCodeHooks(first, "/bin/rmb hook-submit --source=zcode")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured on second pass")
	}
	if strings.Count(second, "hook-submit --source=zcode") != 1 {
		t.Fatalf("expected one rmb hook entry, got: %s", second)
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
	disabled := strings.Replace(zcodeConfigSample, `"enabled": true,`, `"enabled": false,`, 1)
	if zcodeHookConfigured(disabled) {
		t.Fatal("expected not configured when hooks.enabled is false")
	}
}

func TestPreviewZCodeDetected(t *testing.T) {
	fakeRMBHome(t)
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
