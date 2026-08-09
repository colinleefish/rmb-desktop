package setup

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

const testRMBCmd = "/home/user/.rmb/bin/rmb hook-submit --source=cursor"

// cursorHookFixtures are realistic hooks.json shapes from Cursor docs and common setups.
var cursorHookFixtures = []struct {
	name    string
	current string
	exists  bool
	// wantConfigured: after merge, configured flag from mergeCursorHooks
	wantConfigured bool
	// wantChange: expected artifact change type
	wantChange ChangeType
	// wantRMBInStop: number of rmb hook-submit entries in hooks.stop after merge
	wantRMBInStop int
	// mustPreserve substrings that must survive merge
	mustPreserve []string
}{
	{
		name:           "empty file creates stop hook",
		current:        "",
		exists:         false,
		wantConfigured: true,
		wantChange:     ChangeCreate,
		wantRMBInStop:  1,
	},
	{
		name: "docs quickstart user hooks",
		current: `{
  "version": 1,
  "hooks": {
    "afterFileEdit": [{ "command": "./hooks/format.sh" }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"afterFileEdit"`, `"./hooks/format.sh"`},
	},
	{
		name: "docs full multi-hook example",
		current: `{
  "version": 1,
  "hooks": {
    "sessionStart": [{ "command": "./hooks/session-init.sh" }],
    "sessionEnd": [{ "command": "./hooks/audit.sh" }],
    "beforeShellExecution": [
      { "command": "./hooks/audit.sh" },
      { "command": "./hooks/block-git.sh" }
    ],
    "beforeMCPExecution": [{ "command": "./hooks/audit.sh" }],
    "afterShellExecution": [{ "command": "./hooks/audit.sh" }],
    "afterMCPExecution": [{ "command": "./hooks/audit.sh" }],
    "afterFileEdit": [{ "command": "./hooks/audit.sh" }],
    "beforeSubmitPrompt": [{ "command": "./hooks/audit.sh" }],
    "preCompact": [{ "command": "./hooks/audit.sh" }],
    "stop": [{ "command": "./hooks/audit.sh" }],
    "beforeTabFileRead": [{ "command": "./hooks/redact-secrets-tab.sh" }],
    "afterTabFileEdit": [{ "command": "./hooks/format-tab.sh" }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve: []string{
			`"sessionStart"`, `"beforeShellExecution"`, `"afterTabFileEdit"`,
			`"./hooks/audit.sh"`, `"./hooks/block-git.sh"`,
		},
	},
	{
		name: "command hook with matcher timeout failClosed",
		current: `{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "command": "./scripts/approve-network.sh",
        "timeout": 30,
        "matcher": "curl|wget|nc"
      }
    ],
    "preToolUse": [
      {
        "command": "./hooks/validate-tool.sh",
        "matcher": "Shell|Read|Write"
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"matcher"`, `"curl|wget|nc"`, `"Shell|Read|Write"`},
	},
	{
		name: "prompt-based hook untouched",
		current: `{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [
      {
        "type": "prompt",
        "prompt": "Does this command look safe to execute?",
        "timeout": 10
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"type": "prompt"`, `"prompt"`},
	},
	{
		name: "existing stop audit only appends rmb",
		current: `{
  "version": 1,
  "hooks": {
    "stop": [
      { "command": "./hooks/audit.sh", "loop_limit": 10 }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"./hooks/audit.sh"`, `"loop_limit"`},
	},
	{
		name: "typescript bun stop hook preserved",
		current: `{
  "version": 1,
  "hooks": {
    "stop": [
      { "command": "bun run .cursor/hooks/track-stop.ts --stop" }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`track-stop.ts`},
	},
	{
		name: "rmb already configured unchanged",
		current: `{
  "version": 1,
  "hooks": {
    "stop": [
      { "command": "/home/user/.rmb/bin/rmb hook-submit --source=cursor", "timeout": 15 }
    ],
    "afterFileEdit": [{ "command": "./hooks/format.sh" }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeUnchanged,
		wantRMBInStop:  1,
		mustPreserve:   []string{`hook-submit --source=cursor`, `"afterFileEdit"`},
	},
	{
		name: "rmb without timeout gets timeout added",
		current: `{
  "version": 1,
  "hooks": {
    "stop": [{ "command": "/usr/local/bin/rmb hook-submit --source=cursor" }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
	},
	{
		name: "rmb in home path variant",
		current: `{
  "version": 1,
  "hooks": {
    "stop": [{ "command": "~/.rmb/bin/rmb hook-submit --source=cursor", "timeout": 15 }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeUnchanged,
		wantRMBInStop:  1,
	},
	{
		name: "multiple stop hooks one rmb no duplicate",
		current: `{
  "version": 1,
  "hooks": {
    "stop": [
      { "command": "./hooks/audit.sh" },
      { "command": "/home/user/.rmb/bin/rmb hook-submit --source=cursor", "timeout": 15 },
      { "command": "./hooks/cleanup.sh" }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeUnchanged,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"./hooks/audit.sh"`, `"./hooks/cleanup.sh"`},
	},
	{
		name: "missing version added",
		current: `{
  "hooks": {
    "afterFileEdit": [{ "command": "./format.sh" }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"afterFileEdit"`},
	},
	{
		name: "empty hooks object",
		current: `{
  "version": 1,
  "hooks": {}
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
	},
	{
		name: "empty stop array",
		current: `{
  "version": 1,
  "hooks": {
    "stop": []
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
	},
	{
		name: "workspaceOpen lifecycle hook preserved",
		current: `{
  "version": 1,
  "hooks": {
    "workspaceOpen": [{ "command": "./register-workspace-plugins.sh" }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"workspaceOpen"`},
	},
	{
		name: "hook-submit in non-stop does not count as configured",
		current: `{
  "version": 1,
  "hooks": {
    "afterFileEdit": [{ "command": "/usr/local/bin/rmb hook-submit --source=cursor" }]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"afterFileEdit"`},
	},
}

func TestMergeCursorHooksDocFixtures(t *testing.T) {
	for _, tc := range cursorHookFixtures {
		t.Run(tc.name, func(t *testing.T) {
			proposed, configured, err := mergeCursorHooks(tc.current, testRMBCmd)
			if err != nil {
				t.Fatalf("merge error: %v", err)
			}
			if configured != tc.wantConfigured {
				t.Fatalf("configured=%v want %v\nproposed=%s", configured, tc.wantConfigured, proposed)
			}
			change := changeTypeForJSON(tc.current, proposed, tc.exists)
			if change != tc.wantChange {
				t.Fatalf("change=%s want %s\nproposed=%s", change, tc.wantChange, proposed)
			}
			if got := countRMBInStop(proposed); got != tc.wantRMBInStop {
				t.Fatalf("rmb in stop=%d want %d\nproposed=%s", got, tc.wantRMBInStop, proposed)
			}
			for _, sub := range tc.mustPreserve {
				if !strings.Contains(proposed, sub) {
					t.Fatalf("lost %q in proposed:\n%s", sub, proposed)
				}
			}
			if !json.Valid([]byte(proposed)) {
				t.Fatalf("invalid json: %s", proposed)
			}
			// configured status from file content (preview API uses this)
			hookConfigured := cursorHookConfigured(tc.current)
			if tc.wantChange == ChangeUnchanged && strings.Contains(tc.current, "hook-submit --source=cursor") {
				if !hookConfigured {
					t.Fatal("cursorHookConfigured should be true for existing rmb stop hook")
				}
			}
		})
	}
}

func TestCursorHookConfiguredOnlyStop(t *testing.T) {
	withStop := `{"version":1,"hooks":{"stop":[{"command":"/bin/rmb hook-submit --source=cursor"}]}}`
	withEdit := `{"version":1,"hooks":{"afterFileEdit":[{"command":"/bin/rmb hook-submit --source=cursor"}]}}`
	if !cursorHookConfigured(withStop) {
		t.Fatal("expected configured when rmb in stop")
	}
	if cursorHookConfigured(withEdit) {
		t.Fatal("should not be configured when rmb only in afterFileEdit")
	}
}

func TestMergeCursorHooksRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	hookNames := []string{
		"sessionStart", "sessionEnd", "preToolUse", "postToolUse", "postToolUseFailure",
		"subagentStart", "subagentStop", "beforeShellExecution", "afterShellExecution",
		"beforeMCPExecution", "afterMCPExecution", "beforeReadFile", "afterFileEdit",
		"beforeSubmitPrompt", "preCompact", "stop", "afterAgentResponse", "afterAgentThought",
		"beforeTabFileRead", "afterTabFileEdit", "workspaceOpen",
	}
	commands := []string{
		"./hooks/audit.sh",
		"./hooks/format.sh",
		"python3 .cursor/hooks/kube_guard.py",
		"bun run .cursor/hooks/track-stop.ts --stop",
		"/usr/local/bin/rmb hook-submit --source=cursor",
		"~/.rmb/bin/rmb hook-submit --source=cursor",
	}

	for i := 0; i < 200; i++ {
		hooks := map[string][]map[string]any{}
		nEvents := 1 + rng.Intn(8)
		picked := map[string]bool{}
		hasRMBInStop := false
		for j := 0; j < nEvents; j++ {
			name := hookNames[rng.Intn(len(hookNames))]
			if picked[name] {
				continue
			}
			picked[name] = true
			nEntries := 1 + rng.Intn(2)
			var entries []map[string]any
			for k := 0; k < nEntries; k++ {
				cmd := commands[rng.Intn(len(commands))]
				entry := map[string]any{"command": cmd}
				if rng.Intn(3) == 0 {
					entry["timeout"] = 15 + rng.Intn(30)
				}
				if rng.Intn(4) == 0 {
					entry["matcher"] = "Shell|Read"
				}
				if rng.Intn(5) == 0 {
					entry["loop_limit"] = 10
				}
				if name == "beforeShellExecution" && rng.Intn(6) == 0 {
					entry = map[string]any{
						"type":    "prompt",
						"prompt":  "Is this safe?",
						"timeout": 10,
					}
				}
				entries = append(entries, entry)
				if name == "stop" && isRMBHookCommand(cmd) {
					hasRMBInStop = true
				}
			}
			hooks[name] = entries
		}
		root := map[string]any{"hooks": hooks}
		if rng.Intn(2) == 0 {
			root["version"] = 1
		}
		raw, err := json.Marshal(root)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}
		current := string(raw)

		proposed, configured, err := mergeCursorHooks(current, testRMBCmd)
		if err != nil {
			t.Fatalf("case %d merge: %v\ncurrent=%s", i, err, current)
		}
		if !configured {
			t.Fatalf("case %d: expected configured\ncurrent=%s\nproposed=%s", i, current, proposed)
		}
		if !json.Valid([]byte(proposed)) {
			t.Fatalf("case %d: invalid json: %s", i, proposed)
		}
		rmbCount := countRMBInStop(proposed)
		if hasRMBInStop {
			if rmbCount < 1 {
				t.Fatalf("case %d: lost rmb stop hook\ncurrent=%s\nproposed=%s", i, current, proposed)
			}
		} else if rmbCount != 1 {
			t.Fatalf("case %d: want exactly 1 rmb in stop, got %d\ncurrent=%s\nproposed=%s", i, rmbCount, current, proposed)
		}
		for name, entries := range hooks {
			if name == "stop" {
				continue
			}
			for _, entry := range entries {
				if cmd, ok := entry["command"].(string); ok && cmd != "" {
					if !strings.Contains(proposed, cmd) {
						t.Fatalf("case %d: lost command %q from %s\ncurrent=%s\nproposed=%s", i, cmd, name, current, proposed)
					}
				}
				if prompt, ok := entry["prompt"].(string); ok && prompt != "" {
					if !strings.Contains(proposed, prompt) {
						t.Fatalf("case %d: lost prompt from %s", i, name)
					}
				}
			}
		}
		// Non-RMB stop entries preserved
		if stopEntries, ok := hooks["stop"]; ok {
			for _, entry := range stopEntries {
				cmd, _ := entry["command"].(string)
				if cmd != "" && !isRMBHookCommand(cmd) && !strings.Contains(proposed, cmd) {
					t.Fatalf("case %d: lost non-rmb stop command %q", i, cmd)
				}
			}
		}
	}
}

func countRMBInStop(raw string) int {
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return 0
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return 0
	}
	stop, _ := hooks["stop"].([]any)
	n := 0
	for _, item := range stop {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := m["command"].(string)
		if isRMBHookCommand(cmd) {
			n++
		}
	}
	return n
}
