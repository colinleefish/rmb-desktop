package setup

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

const testCodexRMBCmd = "/home/user/.rmb/bin/rmb hook-submit --source=codex"

var codexHookFixtures = []struct {
	name           string
	current        string
	exists         bool
	wantConfigured bool
	wantChange     ChangeType
	wantRMBInStop  int
	mustPreserve   []string
}{
	{
		name:           "empty file creates hooks.Stop",
		current:        "",
		exists:         false,
		wantConfigured: true,
		wantChange:     ChangeCreate,
		wantRMBInStop:  1,
	},
	{
		name: "docs full hooks.json example without Stop",
		current: `{
  "description": "Optional lifecycle hooks for this workspace.",
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume",
        "hooks": [
          {
            "type": "command",
            "command": "python3 ~/.codex/hooks/session_start.py",
            "statusMessage": "Loading session notes",
            "additionalContextLimit": 5000
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/bin/python3 \"$(git rev-parse --show-toplevel)/.codex/hooks/pre_tool_use_policy.py\"",
            "statusMessage": "Checking Bash command"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/bin/python3 \"$(git rev-parse --show-toplevel)/.codex/hooks/post_tool_use_review.py\"",
            "statusMessage": "Reviewing Bash output"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/bin/python3 \"$(git rev-parse --show-toplevel)/.codex/hooks/user_prompt_submit_data_flywheel.py\""
          }
        ]
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve: []string{
			`"description"`, `"SessionStart"`, `"PreToolUse"`, `"PostToolUse"`,
			`"UserPromptSubmit"`, `session_start.py`, `pre_tool_use_policy.py`,
			`"statusMessage"`, `"additionalContextLimit"`,
		},
	},
	{
		name: "existing stop command hook preserved",
		current: `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/bin/python3 \"$(git rev-parse --show-toplevel)/.codex/hooks/stop_continue.py\"",
            "timeout": 30
          }
        ]
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`stop_continue.py`, `"timeout": 30`},
	},
	{
		name: "rmb already configured unchanged",
		current: `{
  "description": "Workspace hooks",
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/.rmb/bin/rmb hook-submit --source=codex",
            "timeout": 15
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./hooks/policy.py"
          }
        ]
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeUnchanged,
		wantRMBInStop:  1,
		mustPreserve:   []string{`hook-submit --source=codex`, `"PreToolUse"`, `policy.py`},
	},
	{
		name: "rmb without timeout gets timeout added",
		current: `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/rmb hook-submit --source=codex"
          }
        ]
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
	},
	{
		name: "multiple stop groups one rmb no duplicate",
		current: `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "./hooks/stop_continue.py"
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/.rmb/bin/rmb hook-submit --source=codex",
            "timeout": 15
          }
        ]
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeUnchanged,
		wantRMBInStop:  1,
		mustPreserve:   []string{`stop_continue.py`},
	},
	{
		name: "hook-submit in pretooluse does not count as configured",
		current: `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/rmb hook-submit --source=codex"
          }
        ]
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"PreToolUse"`},
	},
	{
		name: "empty hooks object",
		current: `{
  "description": "Workspace hooks"
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"description"`},
	},
	{
		name: "empty Stop array",
		current: `{
  "hooks": {
    "Stop": []
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
	},
	{
		name: "legacy top-level Stop migrated into hooks.Stop",
		current: `{
  "description": "Legacy layout",
  "Stop": [
    {
      "hooks": [
        {
          "type": "command",
          "command": "./hooks/stop_continue.py",
          "timeout": 30
        }
      ]
    }
  ]
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"description"`, `stop_continue.py`},
	},
	{
		name: "sessionend hook preserved",
		current: `{
  "hooks": {
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "python3 ~/.codex/hooks/session_end.py",
            "timeout": 3
          }
        ]
      }
    ]
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"SessionEnd"`, `session_end.py`},
	},
}

func TestMergeCodexHooksDocFixtures(t *testing.T) {
	for _, tc := range codexHookFixtures {
		t.Run(tc.name, func(t *testing.T) {
			proposed, configured, err := mergeCodexHooks(tc.current, testCodexRMBCmd)
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
			if got := countRMBInCodexStop(proposed); got != tc.wantRMBInStop {
				t.Fatalf("rmb in Stop=%d want %d\nproposed=%s", got, tc.wantRMBInStop, proposed)
			}
			for _, sub := range tc.mustPreserve {
				if !strings.Contains(proposed, sub) {
					t.Fatalf("lost %q in proposed:\n%s", sub, proposed)
				}
			}
			if tc.name == "legacy top-level Stop migrated into hooks.Stop" {
				var root map[string]any
				if err := json.Unmarshal([]byte(proposed), &root); err != nil {
					t.Fatalf("invalid json: %v", err)
				}
				if _, ok := root["Stop"]; ok {
					t.Fatalf("expected legacy top-level Stop to be removed:\n%s", proposed)
				}
			}
			if strings.TrimSpace(proposed) != "" && !json.Valid([]byte(proposed)) {
				t.Fatalf("invalid json: %s", proposed)
			}
			if !strings.Contains(proposed, `"hooks"`) {
				t.Fatalf("expected hooks wrapper in proposed:\n%s", proposed)
			}
			if tc.wantChange == ChangeUnchanged && strings.Contains(tc.current, "hook-submit --source=codex") {
				if !codexHookConfigured(tc.current) {
					t.Fatal("codexHookConfigured should be true for existing rmb Stop hook")
				}
			}
		})
	}
}

func TestCodexHookConfiguredOnlyStop(t *testing.T) {
	withStop := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/bin/rmb hook-submit --source=codex"}]}]}}`
	withPreToolUse := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/bin/rmb hook-submit --source=codex"}]}]}}`
	legacyStop := `{"Stop":[{"hooks":[{"type":"command","command":"/bin/rmb hook-submit --source=codex"}]}]}`
	if !codexHookConfigured(withStop) {
		t.Fatal("expected configured when rmb in hooks.Stop")
	}
	if codexHookConfigured(withPreToolUse) {
		t.Fatal("should not be configured when rmb only in PreToolUse")
	}
	if !codexHookConfigured(legacyStop) {
		t.Fatal("expected configured for legacy top-level Stop")
	}
}

func TestMergeCodexHooksIdempotent(t *testing.T) {
	first, _, err := mergeCodexHooks("", testCodexRMBCmd)
	if err != nil {
		t.Fatal(err)
	}
	second, configured, err := mergeCodexHooks(first, testCodexRMBCmd)
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured on second pass")
	}
	if countRMBInCodexStop(second) != 1 {
		t.Fatalf("expected exactly one rmb Stop hook, got: %s", second)
	}
	if changeTypeForJSON(first, second, true) != ChangeUnchanged {
		t.Fatalf("expected unchanged on second pass: %s", second)
	}
}

func TestMergeCodexHooksRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(77))
	hookEvents := []string{
		"SessionStart", "SessionEnd", "PreToolUse", "PermissionRequest", "PostToolUse",
		"PreCompact", "PostCompact", "UserPromptSubmit", "SubagentStart", "SubagentStop", "Stop",
	}
	matchers := []string{"", "Bash", "Edit|Write", "startup|resume", "manual|auto"}
	commands := []string{
		"./hooks/audit.sh",
		"python3 ~/.codex/hooks/session_start.py",
		"/usr/local/bin/rmb hook-submit --source=codex",
		"~/.rmb/bin/rmb hook-submit --source=codex",
	}

	for i := 0; i < 200; i++ {
		hooks := map[string][]map[string]any{}
		nEvents := 1 + rng.Intn(7)
		picked := map[string]bool{}
		hasRMBInStop := false

		for j := 0; j < nEvents; j++ {
			event := hookEvents[rng.Intn(len(hookEvents))]
			if picked[event] {
				continue
			}
			picked[event] = true

			group := map[string]any{}
			if rng.Intn(3) != 0 {
				group["matcher"] = matchers[rng.Intn(len(matchers))]
			}
			nHandlers := 1 + rng.Intn(2)
			var handlers []map[string]any
			for k := 0; k < nHandlers; k++ {
				cmd := commands[rng.Intn(len(commands))]
				handler := map[string]any{
					"type":    "command",
					"command": cmd,
				}
				if rng.Intn(3) == 0 {
					handler["timeout"] = 15 + rng.Intn(30)
				}
				if rng.Intn(4) == 0 {
					handler["statusMessage"] = "Running hook"
				}
				if event == "Stop" && isRMBHookCommand(cmd) {
					hasRMBInStop = true
				}
				handlers = append(handlers, handler)
			}
			group["hooks"] = handlers
			hooks[event] = []map[string]any{group}
		}

		root := map[string]any{
			"description": "Generated hooks",
			"hooks":       hooks,
		}
		if rng.Intn(5) == 0 {
			delete(root, "description")
		}

		raw, err := json.Marshal(root)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}
		current := string(raw)

		proposed, configured, err := mergeCodexHooks(current, testCodexRMBCmd)
		if err != nil {
			t.Fatalf("case %d merge: %v\ncurrent=%s", i, err, current)
		}
		if !configured {
			t.Fatalf("case %d: expected configured\ncurrent=%s\nproposed=%s", i, current, proposed)
		}
		if !json.Valid([]byte(proposed)) {
			t.Fatalf("case %d: invalid json: %s", i, proposed)
		}

		rmbCount := countRMBInCodexStop(proposed)
		if hasRMBInStop {
			if rmbCount < 1 {
				t.Fatalf("case %d: lost rmb Stop hook\ncurrent=%s\nproposed=%s", i, current, proposed)
			}
		} else if rmbCount != 1 {
			t.Fatalf("case %d: want exactly 1 rmb in Stop, got %d\ncurrent=%s\nproposed=%s", i, rmbCount, current, proposed)
		}

		for event, groups := range hooks {
			if event == "Stop" {
				continue
			}
			for _, group := range groups {
				inner, _ := group["hooks"].([]map[string]any)
				for _, handler := range inner {
					cmd, _ := handler["command"].(string)
					if cmd != "" && !strings.Contains(proposed, cmd) {
						t.Fatalf("case %d: lost command %q from %s", i, cmd, event)
					}
				}
			}
		}
	}
}

func countRMBInCodexStop(raw string) int {
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return 0
	}
	n := 0
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
				n++
			}
		}
	}
	return n
}
