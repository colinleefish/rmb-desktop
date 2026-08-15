package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const workbuddySampleTranscript = `{"id":"u1","timestamp":1,"type":"message","role":"user","content":[{"type":"input_text","text":"<system-reminder data-role=\"user-context\">\n<user_info>\nOS Version: darwin\n</user_info>\n<identity_context>\nInjected workspace identity files:\n\n## SOUL.md\nPath: /Users/liguanghui/.workbuddy/SOUL.md\n---\n# SOUL.md - Who You Are\n</identity_context>\n</system-reminder>\n<user_query>summarize the attendance notice</user_query>"}]}
{"id":"s1","timestamp":2,"type":"file-history-snapshot","snapshot":{"files":[]}}
{"id":"a1","timestamp":3,"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello! How can I help?"}]}
{"id":"u2","timestamp":4,"type":"message","role":"user","content":[{"type":"input_text","text":"plain follow-up question"}]}
{"id":"r1","timestamp":5,"type":"reasoning","content":[{"type":"text","text":"need to read the file"}]}
{"id":"f1","timestamp":6,"type":"function_call","name":"Read","arguments":"{\"path\":\"/tmp/att.md\"}"}
{"id":"f2","timestamp":7,"type":"function_call_result","name":"Read","status":"completed","output":{"type":"text","text":"file contents"}}
{"id":"a2","timestamp":8,"type":"message","role":"assistant","content":[{"type":"output_text","text":"Here is the summary."}]}
`

func workbuddyFixtureTranscript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(workbuddySampleTranscript), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIsWorkBuddyPayload(t *testing.T) {
	home, _ := os.UserHomeDir()
	transcript := filepath.Join(home, ".workbuddy", "projects", "proj", "sess.jsonl")

	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "workbuddy transcript path",
			raw:  `{"session_id":"abc","transcript_path":"` + transcript + `","cwd":"/tmp","hook_event_name":"Stop"}`,
			want: true,
		},
		{
			name: "bare stop event (workbuddy-shaped)",
			raw:  `{"session_id":"abc","hook_event_name":"Stop","stop_hook_active":true}`,
			want: true,
		},
		{
			name: "foreign transcript path is rejected",
			raw:  `{"session_id":"abc","transcript_path":"/tmp/x.jsonl","cwd":"/tmp","hook_event_name":"Stop","stop_hook_active":true}`,
			want: false,
		},
		{
			name: "codex shaped (has model) is rejected",
			raw:  `{"session_id":"abc","transcript_path":"/tmp/x.jsonl","cwd":"/tmp","hook_event_name":"Stop","model":"gpt-5","turn_id":"t1"}`,
			want: false,
		},
		{
			name: "claude path is rejected",
			raw:  `{"session_id":"abc","transcript_path":"` + filepath.Join(home, ".claude", "s.jsonl") + `","cwd":"/tmp","hook_event_name":"Stop"}`,
			want: false,
		},
		{
			name: "empty is rejected",
			raw:  ``,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsWorkBuddyPayload([]byte(tc.raw)); got != tc.want {
				t.Fatalf("IsWorkBuddyPayload() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseWorkBuddyPayload(t *testing.T) {
	transcript := workbuddyFixtureTranscript(t)
	raw := `{"session_id":"abc-123","transcript_path":"` + transcript + `","cwd":"/tmp","hook_event_name":"Stop","stop_hook_active":true}`

	sessionKey, messages, reason, err := ParseWorkBuddyPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if sessionKey != "abc-123" {
		t.Fatalf("sessionKey = %q, want abc-123", sessionKey)
	}
	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2 (reason=%s)", len(messages), reason)
	}
	if messages[0].Role != "user" || !strings.Contains(messages[0].Content, "plain follow-up question") {
		t.Fatalf("user message wrong: %+v", messages[0])
	}
	if strings.Contains(messages[0].Content, "system-reminder") {
		t.Fatalf("user message leaked injected context: %q", messages[0].Content)
	}
	if messages[1].Role != "assistant" || !strings.Contains(messages[1].Content, "Here is the summary.") {
		t.Fatalf("assistant message wrong: %+v", messages[1])
	}
}

func TestParseWorkBuddyPayloadUserQueryExtraction(t *testing.T) {
	transcript := workbuddyFixtureTranscript(t)
	raw := `{"session_id":"abc","transcript_path":"` + transcript + `","cwd":"/tmp","hook_event_name":"Stop"}`

	_, messages, _, err := ParseWorkBuddyPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	// Last real user turn is the plain follow-up; the earlier system-reminder
	// turn must be skipped entirely (including its injected identity files).
	if messages[0].Role != "user" || messages[0].Content != "plain follow-up question" {
		t.Fatalf("user message wrong: %+v", messages[0])
	}
	if strings.Contains(messages[0].Content, "system-reminder") || strings.Contains(messages[0].Content, "SOUL.md") {
		t.Fatalf("user message leaked injected context: %q", messages[0].Content)
	}
}

func TestWorkBuddyUserQueryInSystemReminderBlock(t *testing.T) {
	text := "<system-reminder data-role=\"user-context\">\n<user_info>OS: darwin</user_info>\n</system-reminder>\n<user_query>聊一聊孟德斯鸠</user_query>"
	if got := workbuddyUserQuery(text); got != "聊一聊孟德斯鸠" {
		t.Fatalf("workbuddyUserQuery() = %q", got)
	}
}

func TestWorkBuddyUserQueryAbsent(t *testing.T) {
	if got := workbuddyUserQuery("<system-reminder data-role=\"user-context\">...</system-reminder>"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := workbuddyUserQuery("plain text"); got != "" {
		t.Fatalf("expected empty for plain text, got %q", got)
	}
}
func TestParseWorkBuddyPayloadFallsBackToPayloadAssistant(t *testing.T) {
	transcript := workbuddyFixtureTranscript(t)
	raw := `{"session_id":"abc","transcript_path":"` + transcript + `","cwd":"/tmp","hook_event_name":"Stop","last_assistant_message":"fallback reply"}`

	_, messages, _, err := ParseWorkBuddyPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	// Transcript has an assistant message, so it wins over the payload field.
	if !strings.Contains(messages[len(messages)-1].Content, "Here is the summary.") {
		t.Fatalf("expected transcript assistant message, got %+v", messages)
	}
}

func TestParseWorkBuddyPayloadMissingAssistant(t *testing.T) {
	raw := `{"session_id":"abc","transcript_path":"/tmp/missing.jsonl","cwd":"/tmp","hook_event_name":"Stop"}`
	if _, _, _, err := ParseWorkBuddyPayload([]byte(raw)); err == nil {
		t.Fatal("expected error for missing assistant message")
	}
}

func TestParseWorkBuddyPayloadMissingSession(t *testing.T) {
	raw := `{"transcript_path":"/tmp/missing.jsonl","cwd":"/tmp","hook_event_name":"Stop"}`
	if _, _, _, err := ParseWorkBuddyPayload([]byte(raw)); err == nil {
		t.Fatal("expected error for missing session_id")
	}
}
