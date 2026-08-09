package setup

import (
	"strings"
	"testing"
)

func TestMergeCursorHooksCreatesStopHook(t *testing.T) {
	proposed, configured, err := mergeCursorHooks(`{
  "version": 1,
  "hooks": {
    "afterFileEdit": [{"command": "./hooks/format.sh"}]
  }
}`, "/usr/local/bin/rmb hook-submit --source=cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, `"stop"`) {
		t.Fatalf("missing stop hook: %s", proposed)
	}
	if !strings.Contains(proposed, `hook-submit --source=cursor`) {
		t.Fatalf("missing rmb command: %s", proposed)
	}
	if !strings.Contains(proposed, `afterFileEdit`) {
		t.Fatalf("lost existing hook: %s", proposed)
	}
}

func TestMergeCursorHooksIdempotent(t *testing.T) {
	first, _, err := mergeCursorHooks("", "/bin/rmb hook-submit --source=cursor")
	if err != nil {
		t.Fatal(err)
	}
	second, configured, err := mergeCursorHooks(first, "/bin/rmb hook-submit --source=cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured on second pass")
	}
	if strings.Count(second, `"command"`) != 1 {
		t.Fatalf("expected one command entry, got: %s", second)
	}
}

func TestMergeClaudeSettingsPreservesEnv(t *testing.T) {
	proposed, configured, err := mergeClaudeSettings(`{
  "env": {"ANTHROPIC_API_KEY": "secret"},
  "model": "sonnet"
}`, "/bin/rmb hook-submit --source=cc")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, `"ANTHROPIC_API_KEY"`) {
		t.Fatalf("lost env: %s", proposed)
	}
	if !strings.Contains(proposed, `"Stop"`) {
		t.Fatalf("missing Stop hook: %s", proposed)
	}
}

func TestCursorRecallRuleFormat(t *testing.T) {
	rule := CursorRecallRule()
	if !strings.Contains(rule, "description: Use of rmb command line") {
		t.Fatalf("missing description frontmatter: %s", rule)
	}
	if !strings.Contains(rule, "alwaysApply: true") {
		t.Fatalf("missing alwaysApply: %s", rule)
	}
	if !strings.Contains(rule, "ALWAYS RUN `rmb` cli") {
		t.Fatalf("missing recall body: %s", rule)
	}
}

func TestMergeRecallMarkdownAppendsBlock(t *testing.T) {
	proposed, change := mergeRecallMarkdown("# Notes\n")
	if change != ChangeAppend {
		t.Fatalf("expected append, got %s", change)
	}
	if !strings.Contains(proposed, recallMarkdownHeading) {
		t.Fatalf("missing recall heading: %s", proposed)
	}
	if !strings.Contains(proposed, "ALWAYS RUN `rmb` cli") {
		t.Fatalf("missing recall body: %s", proposed)
	}
}

func TestMergeRecallMarkdownUnchangedWhenBlockPresent(t *testing.T) {
	current := "# Notes\n\n" + recallBlock()
	proposed, change := mergeRecallMarkdown(current)
	if change != ChangeUnchanged {
		t.Fatalf("expected unchanged, got %s\nproposed=%s", change, proposed)
	}
}

func TestHasRecallBlockLegacyMarkers(t *testing.T) {
	legacy := "# x\n\n" + recallStart + "\nold\n" + recallEnd
	if !hasRecallBlock(legacy) {
		t.Fatal("expected legacy markers to count as configured")
	}
}

func TestMergeCursorHooksPreservesExistingRMBHook(t *testing.T) {
	current := `{
  "version": 1,
  "hooks": {
    "stop": [{"command": "/usr/local/bin/rmb hook-submit --source=cursor", "timeout": 15}]
  }
}`
	proposed, configured, err := mergeCursorHooks(current, "/usr/local/bin/rmb hook-submit --source=cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("expected configured")
	}
	if !strings.Contains(proposed, "hook-submit --source=cursor") {
		t.Fatalf("should preserve existing hook command: %s", proposed)
	}
	if changeTypeForJSON(current, proposed, true) != ChangeUnchanged {
		t.Fatalf("expected unchanged, got modify: %s", proposed)
	}
}

func TestChangeTypeForJSONIgnoresRedactionDisplay(t *testing.T) {
	raw := `{"env":{"ANTHROPIC_API_KEY":"secret"},"hooks":{"Stop":[]},"model":"haiku"}`
	redacted := redactSettingsJSON(raw)
	if changeTypeForJSON(raw, raw, true) != ChangeUnchanged {
		t.Fatal("same raw json should be unchanged")
	}
	if redacted == raw {
		t.Fatal("redaction should change display string")
	}
}

func TestMergeRecallMarkdownIdempotent(t *testing.T) {
	first, _ := mergeRecallMarkdown("")
	second, change := mergeRecallMarkdown(first)
	if change != ChangeUnchanged && change != ChangeModify {
		t.Fatalf("expected unchanged/modify, got %s", change)
	}
	if !strings.Contains(second, recallMarkdownHeading) {
		t.Fatalf("missing recall block: %s", second)
	}
}

func TestRedactSettingsJSON(t *testing.T) {
	out := redactSettingsJSON(`{"env":{"ANTHROPIC_API_KEY":"secret"},"model":"sonnet"}`)
	if strings.Contains(out, "secret") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, "••••••") {
		t.Fatalf("expected redaction marker: %s", out)
	}
}
