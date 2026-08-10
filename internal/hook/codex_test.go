package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsCodexPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			"model field present",
			`{"session_id":"abc","model":"gpt-5","last_assistant_message":"hi"}`,
			true,
		},
		{
			"transcript_path under .codex",
			`{"session_id":"abc","transcript_path":"` + codexHome(t) + `/sessions/test.jsonl"}`,
			true,
		},
		{
			"model empty, no .codex path",
			`{"session_id":"abc","model":"","last_assistant_message":"hi"}`,
			false,
		},
		{
			"claude payload is not codex",
			`{"session_id":"abc","last_assistant_message":"hi","cwd":"/home","transcript_path":"` + claudeHome(t) + `/projects/test.jsonl"}`,
			false,
		},
		{
			"empty payload",
			`{}`,
			false,
		},
		{
			"invalid JSON",
			`not json`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCodexPayload([]byte(tt.raw)); got != tt.want {
				t.Fatalf("IsCodexPayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCodexPayload_WithUserAndAssistant(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	rawTranscript := strings.Join([]string{
		`{"timestamp":"2026-06-22T08:41:34.366Z","type":"session_meta","payload":{"id":"abc"}}`,
		`{"timestamp":"2026-06-22T08:41:34.366Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}`,
		`{"timestamp":"2026-06-22T08:41:34.371Z","type":"event_msg","payload":{"type":"user_message","message":"what is Go?"}}`,
		`{"timestamp":"2026-06-22T08:41:35.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Go is a language."}]}}`,
		`{"timestamp":"2026-06-22T08:41:54.504Z","type":"event_msg","payload":{"type":"user_message","message":"tell me more"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(rawTranscript), 0o600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	payload := map[string]any{
		"session_id":             "019eee7d-df7f-7531-8de7-6335f63f9c80",
		"transcript_path":        transcriptPath,
		"cwd":                    "/tmp",
		"last_assistant_message": "Go is a compiled language.",
		"stop_hook_active":       false,
		"hook_event_name":        "Stop",
		"model":                  "gpt-5",
	}
	raw, _ := json.Marshal(payload)

	sid, msgs, reason, err := ParseCodexPayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "019eee7d-df7f-7531-8de7-6335f63f9c80" {
		t.Fatalf("session_id = %q", sid)
	}
	if reason != "user from transcript + assistant from payload" {
		t.Fatalf("reason = %q", reason)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2; msgs = %v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "tell me more" {
		t.Fatalf("msgs[0] = %#v, want user/tell me more", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Go is a compiled language." {
		t.Fatalf("msgs[1] = %#v, want assistant/Go is a compiled language.", msgs[1])
	}
}

func TestParseCodexPayload_AssistantOnlyWhenNoUser(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	rawTranscript := strings.Join([]string{
		`{"timestamp":"2026-06-22T08:41:34.366Z","type":"session_meta","payload":{"id":"abc"}}`,
		`{"timestamp":"2026-06-22T08:41:34.366Z","type":"event_msg","payload":{"type":"task_started","turn_id":"t1"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(rawTranscript), 0o600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	payload := map[string]any{
		"session_id":             "abc",
		"transcript_path":        transcriptPath,
		"cwd":                    "/tmp",
		"last_assistant_message": "hello",
		"stop_hook_active":       false,
		"model":                  "gpt-5",
	}
	raw, _ := json.Marshal(payload)

	_, msgs, reason, err := ParseCodexPayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason != "last_assistant_message only (no user found)" {
		t.Fatalf("reason = %q", reason)
	}
	if len(msgs) != 1 || msgs[0].Role != "assistant" || msgs[0].Content != "hello" {
		t.Fatalf("msgs = %v", msgs)
	}
}

func TestParseCodexPayload_EmptyAssistantErrors(t *testing.T) {
	payload := map[string]any{
		"session_id":             "abc",
		"transcript_path":        "/nonexistent.jsonl",
		"cwd":                    "/tmp",
		"last_assistant_message": "",
		"model":                  "gpt-5",
	}
	raw, _ := json.Marshal(payload)

	_, _, _, err := ParseCodexPayload(raw)
	if err == nil {
		t.Fatal("expected error when last_assistant_message empty")
	}
	if !strings.Contains(err.Error(), "last_assistant_message is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCodexPayload_MissingSessionID(t *testing.T) {
	payload := map[string]any{
		"session_id":             "",
		"transcript_path":        "/nonexistent.jsonl",
		"last_assistant_message": "hi",
		"model":                  "gpt-5",
	}
	raw, _ := json.Marshal(payload)

	_, _, _, err := ParseCodexPayload(raw)
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
	if !strings.Contains(err.Error(), "missing session_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmit_Codex_UploadsToAPI(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	rawTranscript := strings.Join([]string{
		`{"timestamp":"2026-06-22T08:41:34.371Z","type":"event_msg","payload":{"type":"user_message","message":"hello codex"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(rawTranscript), 0o600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	var gotPath string
	var gotBody struct {
		Source   string `json:"source"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	payload := map[string]any{
		"hook_event_name":        "Stop",
		"session_id":             "019eee7d-df7f-7531-8de7-6335f63f9c80",
		"transcript_path":        transcriptPath,
		"cwd":                    "/tmp",
		"last_assistant_message": "codex reply",
		"stop_hook_active":       false,
		"permission_mode":        "full",
		"model":                  "gpt-5",
		"turn_id":                "turn-123",
	}
	raw, _ := json.Marshal(payload)

	var out bytes.Buffer
	if err := Submit(context.Background(), SubmitInput{
		Source:     "codex",
		StdinJSON:  raw,
		OutputSink: &out,
		BaseURL:    srv.URL,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if gotPath != "/api/v1/sessions/019eee7d-df7f-7531-8de7-6335f63f9c80/upload" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody.Source != "codex" {
		t.Fatalf("source = %q", gotBody.Source)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Content != "hello codex" || gotBody.Messages[1].Content != "codex reply" {
		t.Fatalf("body messages = %v", gotBody.Messages)
	}
	if !strings.Contains(out.String(), "action=upload") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestSubmit_Codex_SkipsNonCodexPayload(t *testing.T) {
	payload := map[string]any{
		"hook_event_name": "Stop",
		"some_field":      "some_value",
	}
	raw, _ := json.Marshal(payload)

	var out bytes.Buffer
	if err := Submit(context.Background(), SubmitInput{
		Source:     "codex",
		StdinJSON:  raw,
		OutputSink: &out,
		BaseURL:    "http://127.0.0.1:1",
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !strings.Contains(out.String(), "action=skip") {
		t.Fatalf("expected skip, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "not a codex payload") {
		t.Fatalf("expected 'not a codex payload', got: %q", out.String())
	}
}

func codexHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return home + "/.codex"
}

func claudeHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return home + "/.claude"
}
