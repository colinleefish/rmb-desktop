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

func TestIsZCodePayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			"transcript_path under .zcode",
			`{"session_id":"abc","transcript_path":"` + zcodeHome(t) + `/projects/test.jsonl","hook_event_name":"Stop"}`,
			true,
		},
		{
			"last_assistant_message present",
			`{"session_id":"abc","last_assistant_message":"hi","hook_event_name":"Stop"}`,
			true,
		},
		{
			"stop_hook_active present with empty fields",
			`{"session_id":"abc","stop_hook_active":true}`,
			true,
		},
		{
			"claude path is not zcode",
			`{"session_id":"abc","transcript_path":"` + claudeHome(t) + `/projects/test.jsonl"}`,
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
			if got := IsZCodePayload([]byte(tt.raw)); got != tt.want {
				t.Fatalf("IsZCodePayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseZCodePayload_WithUserAndAssistant(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	rawTranscript := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"what is Go?"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Go is a language."}]}}`,
		`{"type":"user","message":{"role":"user","content":"tell me more"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(rawTranscript), 0o600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	payload := map[string]any{
		"session_id":             "abc-123",
		"transcript_path":        transcriptPath,
		"cwd":                    "/tmp",
		"last_assistant_message": "Go is a compiled language.",
		"stop_hook_active":       false,
		"hook_event_name":        "Stop",
	}
	raw, _ := json.Marshal(payload)

	sid, msgs, reason, err := ParseZCodePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "abc-123" {
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

func TestParseZCodePayload_AssistantOnlyWhenNoUser(t *testing.T) {
	payload := map[string]any{
		"session_id":             "abc",
		"transcript_path":        "/nonexistent.jsonl",
		"cwd":                    "/tmp",
		"last_assistant_message": "hello",
		"stop_hook_active":       false,
	}
	raw, _ := json.Marshal(payload)

	_, msgs, reason, err := ParseZCodePayload(raw)
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

func TestParseZCodePayload_EmptyAssistantErrors(t *testing.T) {
	payload := map[string]any{
		"session_id":             "abc",
		"transcript_path":        "/nonexistent.jsonl",
		"last_assistant_message": "",
	}
	raw, _ := json.Marshal(payload)

	_, _, _, err := ParseZCodePayload(raw)
	if err == nil {
		t.Fatal("expected error when last_assistant_message empty")
	}
	if !strings.Contains(err.Error(), "last_assistant_message is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseZCodePayload_MissingSessionID(t *testing.T) {
	payload := map[string]any{
		"session_id":             "",
		"transcript_path":        "/nonexistent.jsonl",
		"last_assistant_message": "hi",
	}
	raw, _ := json.Marshal(payload)

	_, _, _, err := ParseZCodePayload(raw)
	if err == nil {
		t.Fatal("expected error for missing session_id")
	}
	if !strings.Contains(err.Error(), "missing session_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmit_ZCode_UploadsToAPI(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	rawTranscript := `{"type":"user","message":{"role":"user","content":"hello zcode"}}` + "\n"
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
		"session_id":             "abc-123",
		"transcript_path":        transcriptPath,
		"cwd":                    "/tmp",
		"last_assistant_message": "zcode reply",
		"stop_hook_active":       false,
		"permission_mode":        "default",
	}
	raw, _ := json.Marshal(payload)

	var out bytes.Buffer
	if err := Submit(context.Background(), SubmitInput{
		Source:     "zcode",
		StdinJSON:  raw,
		OutputSink: &out,
		BaseURL:    srv.URL,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if gotPath != "/api/v1/sessions/abc-123/upload" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody.Source != "zcode" {
		t.Fatalf("source = %q", gotBody.Source)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Content != "hello zcode" || gotBody.Messages[1].Content != "zcode reply" {
		t.Fatalf("body messages = %v", gotBody.Messages)
	}
	if !strings.Contains(out.String(), "action=upload") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestSubmit_ZCode_SkipsNonZCodePayload(t *testing.T) {
	payload := map[string]any{
		"hook_event_name": "Stop",
		"some_field":      "some_value",
	}
	raw, _ := json.Marshal(payload)

	var out bytes.Buffer
	if err := Submit(context.Background(), SubmitInput{
		Source:     "zcode",
		StdinJSON:  raw,
		OutputSink: &out,
		BaseURL:    "http://127.0.0.1:1",
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !strings.Contains(out.String(), "action=skip") {
		t.Fatalf("expected skip, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "not a zcode payload") {
		t.Fatalf("expected 'not a zcode payload', got: %q", out.String())
	}
}

func zcodeHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return home + "/.zcode"
}
