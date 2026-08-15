package setup

import (
	"strings"
	"testing"
)

const workbuddySettingsSample = `{
  "enabledPlugins": {
    "weixinpay@workbuddy-builtin": true
  },
  "sandbox": {
    "extraAllowWrite": ["~/.dws"]
  },
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "env RMB_URL=https://rmb.colinleefish.com /Users/liguanghui/.rmb/bin/rmb hook-submit --source=workbuddy",
            "timeout": 15
          }
        ]
      }
    ]
  }
}`

func TestMergeWorkBuddyHooksPreservesSettings(t *testing.T) {
	proposed, configured, err := mergeWorkBuddyHooks(workbuddySettingsSample, "/bin/rmb hook-submit --source=workbuddy")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, "weixinpay@workbuddy-builtin") {
		t.Fatalf("lost enabledPlugins: %s", proposed)
	}
	if !strings.Contains(proposed, "~/.dws") {
		t.Fatalf("lost sandbox config: %s", proposed)
	}
	// Existing RMB command must be normalized to the canonical dynamic command:
	// no env prefix / hardcoded URL — the CLI resolves the endpoint itself.
	if strings.Contains(proposed, "RMB_URL") {
		t.Fatalf("RMB_URL env prefix must be stripped: %s", proposed)
	}
	if !strings.Contains(proposed, "\"command\": \"/bin/rmb hook-submit --source=workbuddy\"") {
		t.Fatalf("expected normalized canonical command: %s", proposed)
	}
}

func TestMergeWorkBuddyHooksAddsStopHook(t *testing.T) {
	current := `{
  "enabledPlugins": {"weixinpay@workbuddy-builtin": true}
}`
	proposed, configured, err := mergeWorkBuddyHooks(current, "/bin/rmb hook-submit --source=workbuddy")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, `"Stop"`) {
		t.Fatalf("missing Stop hook: %s", proposed)
	}
	if !strings.Contains(proposed, `hook-submit --source=workbuddy`) {
		t.Fatalf("missing rmb command: %s", proposed)
	}
	if !strings.Contains(proposed, "weixinpay@workbuddy-builtin") {
		t.Fatalf("lost existing settings: %s", proposed)
	}
}

func TestMergeWorkBuddyHooksIdempotent(t *testing.T) {
	first, _, err := mergeWorkBuddyHooks("", "/bin/rmb hook-submit --source=workbuddy")
	if err != nil {
		t.Fatal(err)
	}
	second, configured, err := mergeWorkBuddyHooks(first, "/bin/rmb hook-submit --source=workbuddy")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured on second pass")
	}
	if strings.Count(second, "hook-submit --source=workbuddy") != 1 {
		t.Fatalf("expected one rmb hook entry, got: %s", second)
	}
}

func TestWorkBuddyHookConfigured(t *testing.T) {
	if !workbuddyHookConfigured(workbuddySettingsSample) {
		t.Fatal("expected configured for sample settings")
	}
	if workbuddyHookConfigured(`{"enabledPlugins":{}}`) {
		t.Fatal("expected not configured for empty settings")
	}
}

func TestPreviewWorkBuddyDetected(t *testing.T) {
	def, _ := agentDefByID(AgentWorkBuddy)
	state, err := previewWorkBuddy(def)
	if err != nil {
		t.Fatal(err)
	}
	if state.ID != "workbuddy" {
		t.Fatalf("id = %q", state.ID)
	}
	if len(state.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(state.Artifacts))
	}
	if state.Artifacts[0].ID != "settings" || state.Artifacts[0].Language != "json" {
		t.Fatalf("artifact 0 wrong: %+v", state.Artifacts[0])
	}
	if state.Artifacts[1].ID != "memory_md" || state.Artifacts[1].Language != "markdown" {
		t.Fatalf("artifact 1 wrong: %+v", state.Artifacts[1])
	}
}
