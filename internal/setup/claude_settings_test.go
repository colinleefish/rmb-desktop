package setup

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
)

const testClaudeRMBCmd = "/home/user/.rmb/bin/rmb hook-submit --source=cc"

// claudeSettingsFixtures are realistic settings.json shapes from Claude Code docs.
var claudeSettingsFixtures = []struct {
	name           string
	current        string
	exists         bool
	wantConfigured bool
	wantChange     ChangeType
	wantRMBInStop  int
	mustPreserve   []string
}{
	{
		name:           "empty file creates Stop hook",
		current:        "",
		exists:         false,
		wantConfigured: true,
		wantChange:     ChangeCreate,
		wantRMBInStop:  1,
	},
	{
		name: "docs example settings with permissions env announcements",
		current: `{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": ["Bash(npm run lint)", "Bash(npm run test *)", "Read(~/.zshrc)"],
    "deny": ["Bash(curl *)", "Read(./.env)", "Read(./.env.*)", "Read(./secrets/**)"]
  },
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp"
  },
  "companyAnnouncements": [
    "Welcome to Acme Corp!",
    "Reminder: Code reviews required for all PRs"
  ]
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve: []string{
			`"$schema"`, `"permissions"`, `"Bash(npm run lint)"`, `"Read(./.env)"`,
			`"CLAUDE_CODE_ENABLE_TELEMETRY"`, `"companyAnnouncements"`,
		},
	},
	{
		name: "pretooluse matcher with if condition preserved",
		current: `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "if": "Bash(rm *)",
            "command": "${CLAUDE_PROJECT_DIR}/.claude/hooks/block-rm.sh",
            "args": []
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
		mustPreserve:   []string{`"PreToolUse"`, `"matcher"`, `"Bash(rm *)"`, `block-rm.sh`},
	},
	{
		name: "posttooluse edit write lint hook preserved",
		current: `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/lint-check.sh"
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
		mustPreserve:   []string{`"PostToolUse"`, `"Edit|Write"`, `lint-check.sh`},
	},
	{
		name: "stop prompt hook preserved",
		current: `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "prompt",
            "prompt": "Evaluate if Claude should stop: $ARGUMENTS. Check if all tasks are complete."
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
		mustPreserve:   []string{`"type": "prompt"`, `Evaluate if Claude should stop`},
	},
	{
		name: "stop agent hook preserved",
		current: `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "agent",
            "prompt": "Verify that all unit tests pass. $ARGUMENTS",
            "timeout": 120
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
		mustPreserve:   []string{`"type": "agent"`, `Verify that all unit tests pass`},
	},
	{
		name: "async posttooluse write hook preserved",
		current: `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write",
        "hooks": [
          {
            "type": "command",
            "command": "./hooks/run-tests.sh",
            "async": true,
            "timeout": 120
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
		mustPreserve:   []string{`"async": true`, `run-tests.sh`},
	},
	{
		name: "sessionstart startup hook preserved",
		current: `{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "./hooks/session-init.sh"
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
		mustPreserve:   []string{`"SessionStart"`, `"startup"`, `session-init.sh`},
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
            "command": "./hooks/audit.sh",
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
		mustPreserve:   []string{`"./hooks/audit.sh"`, `"timeout": 30`},
	},
	{
		name: "rmb already configured unchanged",
		current: `{
  "model": "sonnet",
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/.rmb/bin/rmb hook-submit --source=cc",
            "timeout": 15
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write",
        "hooks": [
          {
            "type": "command",
            "command": "./hooks/lint.sh"
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
		mustPreserve:   []string{`hook-submit --source=cc`, `"PostToolUse"`, `lint.sh`},
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
            "command": "/usr/local/bin/rmb hook-submit --source=cc"
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
		name: "rmb home path variant unchanged",
		current: `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.rmb/bin/rmb hook-submit --source=cc",
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
            "command": "./hooks/audit.sh"
          }
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "/home/user/.rmb/bin/rmb hook-submit --source=cc",
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
		mustPreserve:   []string{`"./hooks/audit.sh"`},
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
            "command": "/usr/local/bin/rmb hook-submit --source=cc"
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
  "model": "haiku"
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"model": "haiku"`},
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
		name: "empty Stop group hooks array gets rmb",
		current: `{
  "hooks": {
    "Stop": [
      {
        "hooks": []
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
		name: "sandbox and statusLine preserved",
		current: `{
  "sandbox": {
    "enabled": true,
    "network": {
      "allowedDomains": ["github.com", "*.npmjs.org"]
    }
  },
  "statusLine": {
    "type": "command",
    "command": "~/.claude/statusline.sh"
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"sandbox"`, `"allowedDomains"`, `"statusLine"`, `statusline.sh`},
	},
	{
		name: "enabledPlugins preserved",
		current: `{
  "enabledPlugins": {
    "formatter@acme-tools": true,
    "deployer@acme-tools": true
  }
}`,
		exists:         true,
		wantConfigured: true,
		wantChange:     ChangeModify,
		wantRMBInStop:  1,
		mustPreserve:   []string{`"enabledPlugins"`, `formatter@acme-tools`},
	},
}

func TestMergeClaudeSettingsDocFixtures(t *testing.T) {
	for _, tc := range claudeSettingsFixtures {
		t.Run(tc.name, func(t *testing.T) {
			proposed, configured, err := mergeClaudeSettings(tc.current, testClaudeRMBCmd)
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
			if got := countRMBInClaudeStop(proposed); got != tc.wantRMBInStop {
				t.Fatalf("rmb in Stop=%d want %d\nproposed=%s", got, tc.wantRMBInStop, proposed)
			}
			for _, sub := range tc.mustPreserve {
				if !strings.Contains(proposed, sub) {
					t.Fatalf("lost %q in proposed:\n%s", sub, proposed)
				}
			}
			if strings.TrimSpace(proposed) != "" && !json.Valid([]byte(proposed)) {
				t.Fatalf("invalid json: %s", proposed)
			}
			if tc.wantChange == ChangeUnchanged && strings.Contains(tc.current, "hook-submit --source=cc") {
				if !claudeHookConfigured(tc.current) {
					t.Fatal("claudeHookConfigured should be true for existing rmb Stop hook")
				}
			}
		})
	}
}

func TestClaudeHookConfiguredOnlyStop(t *testing.T) {
	withStop := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/bin/rmb hook-submit --source=cc"}]}]}}`
	withPreToolUse := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/bin/rmb hook-submit --source=cc"}]}]}}`
	if !claudeHookConfigured(withStop) {
		t.Fatal("expected configured when rmb in Stop")
	}
	if claudeHookConfigured(withPreToolUse) {
		t.Fatal("should not be configured when rmb only in PreToolUse")
	}
}

func TestMergeClaudeSettingsPreservesEnvAndModel(t *testing.T) {
	current := `{
  "env": {"ANTHROPIC_API_KEY": "secret", "FOO": "bar"},
  "model": "sonnet",
  "theme": "dark"
}`
	proposed, configured, err := mergeClaudeSettings(current, testClaudeRMBCmd)
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	for _, sub := range []string{`"ANTHROPIC_API_KEY"`, `"FOO"`, `"model": "sonnet"`, `"theme": "dark"`, `"Stop"`} {
		if !strings.Contains(proposed, sub) {
			t.Fatalf("lost %q in proposed:\n%s", sub, proposed)
		}
	}
	if countRMBInClaudeStop(proposed) != 1 {
		t.Fatalf("expected one rmb Stop hook: %s", proposed)
	}
}

func TestMergeClaudeSettingsIdempotent(t *testing.T) {
	first, _, err := mergeClaudeSettings("", testClaudeRMBCmd)
	if err != nil {
		t.Fatal(err)
	}
	second, configured, err := mergeClaudeSettings(first, testClaudeRMBCmd)
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured on second pass")
	}
	if countRMBInClaudeStop(second) != 1 {
		t.Fatalf("expected exactly one rmb Stop hook, got: %s", second)
	}
	if changeTypeForJSON(first, second, true) != ChangeUnchanged {
		t.Fatalf("expected unchanged on second pass: %s", second)
	}
}

func TestMergeClaudeSettingsRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	hookEvents := []string{
		"SessionStart", "SessionEnd", "Setup", "UserPromptSubmit", "UserPromptExpansion",
		"PreToolUse", "PermissionRequest", "PermissionDenied", "PostToolUse", "PostToolUseFailure",
		"PostToolBatch", "Notification", "MessageDisplay", "SubagentStart", "SubagentStop",
		"TaskCreated", "TaskCompleted", "Stop", "StopFailure", "TeammateIdle",
		"InstructionsLoaded", "ConfigChange", "CwdChanged", "DirectoryAdded", "FileChanged",
		"WorktreeCreate", "WorktreeRemove", "PreCompact", "PostCompact", "Elicitation", "ElicitationResult",
	}
	matchers := []string{"", "Bash", "Edit|Write", "startup", "Write", "mcp__memory__.*"}
	commands := []string{
		"./hooks/audit.sh",
		"./hooks/lint-check.sh",
		"${CLAUDE_PROJECT_DIR}/.claude/hooks/block-rm.sh",
		"/usr/local/bin/rmb hook-submit --source=cc",
		"~/.rmb/bin/rmb hook-submit --source=cc",
	}
	handlerTypes := []string{"command", "prompt", "agent"}

	for i := 0; i < 200; i++ {
		hooks := map[string][]map[string]any{}
		nEvents := 1 + rng.Intn(8)
		picked := map[string]bool{}
		hasRMBInStop := false

		for j := 0; j < nEvents; j++ {
			event := hookEvents[rng.Intn(len(hookEvents))]
			if picked[event] {
				continue
			}
			picked[event] = true

			nGroups := 1 + rng.Intn(2)
			var groups []map[string]any
			for g := 0; g < nGroups; g++ {
				group := map[string]any{}
				if rng.Intn(3) != 0 {
					group["matcher"] = matchers[rng.Intn(len(matchers))]
				}
				nHandlers := 1 + rng.Intn(2)
				var handlers []map[string]any
				for k := 0; k < nHandlers; k++ {
					handlerType := handlerTypes[rng.Intn(len(handlerTypes))]
					handler := map[string]any{"type": handlerType}
					switch handlerType {
					case "command":
						cmd := commands[rng.Intn(len(commands))]
						handler["command"] = cmd
						if rng.Intn(3) == 0 {
							handler["timeout"] = 15 + rng.Intn(30)
						}
						if rng.Intn(4) == 0 {
							handler["if"] = "Bash(git *)"
						}
						if rng.Intn(5) == 0 {
							handler["async"] = true
						}
						if event == "Stop" && isRMBHookCommand(cmd) {
							hasRMBInStop = true
						}
					case "prompt", "agent":
						handler["prompt"] = "Evaluate hook input: $ARGUMENTS"
						if rng.Intn(2) == 0 {
							handler["timeout"] = 30
						}
					}
					handlers = append(handlers, handler)
				}
				group["hooks"] = handlers
				groups = append(groups, group)
			}
			hooks[event] = groups
		}

		root := map[string]any{
			"model": "sonnet",
			"hooks": hooks,
		}
		if rng.Intn(2) == 0 {
			root["permissions"] = map[string]any{
				"allow": []string{"Bash(npm run test *)"},
				"deny":  []string{"Read(./.env)"},
			}
		}
		if rng.Intn(2) == 0 {
			root["env"] = map[string]any{"FOO": "bar"}
		}

		raw, err := json.Marshal(root)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}
		current := string(raw)

		proposed, configured, err := mergeClaudeSettings(current, testClaudeRMBCmd)
		if err != nil {
			t.Fatalf("case %d merge: %v\ncurrent=%s", i, err, current)
		}
		if !configured {
			t.Fatalf("case %d: expected configured\ncurrent=%s\nproposed=%s", i, current, proposed)
		}
		if !json.Valid([]byte(proposed)) {
			t.Fatalf("case %d: invalid json: %s", i, proposed)
		}

		rmbCount := countRMBInClaudeStop(proposed)
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
					if cmd, ok := handler["command"].(string); ok && cmd != "" {
						if !strings.Contains(proposed, cmd) {
							t.Fatalf("case %d: lost command %q from %s\ncurrent=%s\nproposed=%s", i, cmd, event, current, proposed)
						}
					}
					if prompt, ok := handler["prompt"].(string); ok && prompt != "" {
						if !strings.Contains(proposed, prompt) {
							t.Fatalf("case %d: lost prompt from %s", i, event)
						}
					}
				}
			}
		}

		if stopGroups, ok := hooks["Stop"]; ok {
			for _, group := range stopGroups {
				inner, _ := group["hooks"].([]map[string]any)
				for _, handler := range inner {
					cmd, _ := handler["command"].(string)
					if cmd != "" && !isRMBHookCommand(cmd) && !strings.Contains(proposed, cmd) {
						t.Fatalf("case %d: lost non-rmb Stop command %q", i, cmd)
					}
					if prompt, ok := handler["prompt"].(string); ok && prompt != "" && !strings.Contains(proposed, prompt) {
						t.Fatalf("case %d: lost non-rmb Stop prompt", i)
					}
				}
			}
		}

		if !strings.Contains(proposed, `"model": "sonnet"`) {
			t.Fatalf("case %d: lost model field", i)
		}
	}
}

func TestMergeClaudeRecallMarkdownFixtures(t *testing.T) {
	fixtures := []struct {
		name        string
		current     string
		wantChange  ChangeType
		wantBlock   bool
		wantHeading bool
	}{
		{
			name:        "empty creates recall block",
			current:     "",
			wantChange:  ChangeCreate,
			wantBlock:   true,
			wantHeading: true,
		},
		{
			name:        "project notes appends block",
			current:     "# Project notes\n\nUse conventional commits for this repo.\n",
			wantChange:  ChangeAppend,
			wantBlock:   true,
			wantHeading: true,
		},
		{
			name:        "already configured unchanged",
			current:     "# Notes\n\n" + recallBlock(),
			wantChange:  ChangeUnchanged,
			wantBlock:   true,
			wantHeading: true,
		},
		{
			name:       "legacy html comment markers detected",
			current:    "# x\n\n" + recallStart + "\nold\n" + recallEnd,
			wantChange: ChangeUnchanged,
			wantBlock:  true,
			wantHeading: false,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			proposed, change := mergeRecallMarkdown(tc.current)
			if change != tc.wantChange {
				t.Fatalf("change=%s want %s\nproposed=%s", change, tc.wantChange, proposed)
			}
			if tc.wantBlock && !hasRecallBlock(proposed) {
				t.Fatalf("expected recall block in proposed:\n%s", proposed)
			}
			if tc.wantHeading && !strings.Contains(proposed, recallMarkdownHeading) {
				t.Fatalf("missing recall heading:\n%s", proposed)
			}
			if tc.wantHeading && !strings.Contains(proposed, "ALWAYS RUN `rmb` cli") {
				t.Fatalf("missing recall body:\n%s", proposed)
			}
		})
	}
}

func countRMBInClaudeStop(raw string) int {
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return 0
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return 0
	}
	stop, _ := hooks["Stop"].([]any)
	n := 0
	for _, item := range stop {
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
