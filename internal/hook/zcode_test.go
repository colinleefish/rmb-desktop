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

// zcodeStopTranscriptLine is the exact single line ZCode writes into the Stop
// hook's temp transcript (verified against ZCode's CLI bundle): assistant
// only, no row "type" field, no user message.
const zcodeStopTranscriptLine = `{"message":{"content":[{"text":"Go is a compiled language.","type":"text"}],"role":"assistant"}}

`

func TestIsZCodePayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
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
			"camel responseText only (no snake alias) is rejected (TC-4)",
			`{"session_id":"abc","responseText":"hi","hook_event_name":"Stop"}`,
			false,
		},
		{
			"temp transcript outside ~/.zcode is not a detection signal",
			`{"session_id":"abc","transcript_path":"/var/folders/xx/zcode-claude-hook-123/transcript.jsonl","hook_event_name":"Stop"}`,
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

func TestParseZCodePayload_PairsCapturedPrompt(t *testing.T) {
	// Keep sidecar writes out of the developer's real ~/.rmb cache.
	t.Setenv("HOME", t.TempDir())

	if err := CaptureZCodePrompt([]byte(`{"session_id":"ABC-123","prompt":"what is Go?","hook_event_name":"UserPromptSubmit"}`)); err != nil {
		t.Fatalf("capture: %v", err)
	}

	// Stop transcript is assistant-only; parsing must not depend on it.
	transcriptPath := filepath.Join(t.TempDir(), "zcode-claude-hook-x", "transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcriptPath, []byte(zcodeStopTranscriptLine), 0o600); err != nil {
		t.Fatal(err)
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
	if reason != "user from prompt capture + assistant from payload" {
		t.Fatalf("reason = %q", reason)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2; msgs = %v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "what is Go?" {
		t.Fatalf("msgs[0] = %#v, want user/what is Go?", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Go is a compiled language." {
		t.Fatalf("msgs[1] = %#v", msgs[1])
	}

	// Sidecar is consumed: a second parse degrades to assistant-only.
	_, msgs, reason, err = ParseZCodePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reason != "last_assistant_message only (no captured prompt)" {
		t.Fatalf("reason = %q", reason)
	}
	if len(msgs) != 1 || msgs[0].Role != "assistant" {
		t.Fatalf("msgs = %v", msgs)
	}
}

func TestParseZCodePayload_AssistantOnlyWhenNoCapture(t *testing.T) {
	payload := map[string]any{
		"session_id":             "no-capture",
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
	if reason != "last_assistant_message only (no captured prompt)" {
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

func TestCaptureZCodePrompt_SkipsIncomplete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cases := []string{
		`{"session_id":"","prompt":"hi"}`,
		`{"session_id":"abc","prompt":""}`,
		`{"session_id":"../escape","prompt":"hi"}`,
		`not json`,
		``,
	}
	for _, raw := range cases {
		if err := CaptureZCodePrompt([]byte(raw)); err != nil {
			t.Fatalf("CaptureZCodePrompt(%q) = %v, want nil", raw, err)
		}
	}
	if _, err := os.Stat(zcodePromptPath("abc")); !os.IsNotExist(err) {
		t.Fatal("expected no sidecar for incomplete payloads")
	}
	// Path-escape guard: a session id with separators must never create a file
	// outside the sidecar dir.
	if _, err := os.Stat(filepath.Join(zcodePromptDir(), "..", "escape.json")); !os.IsNotExist(err) {
		t.Fatal("expected no sidecar escape outside the prompt dir")
	}
}

func TestCaptureZCodePrompt_LastWriteWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_ = CaptureZCodePrompt([]byte(`{"session_id":"queue-1","prompt":"first"}`))
	_ = CaptureZCodePrompt([]byte(`{"session_id":"queue-1","prompt":"second"}`))

	if got := takeCapturedZCodePrompt("queue-1"); got != "second" {
		t.Fatalf("captured prompt = %q, want second", got)
	}
}

func TestSubmit_ZCode_PairsCapturedPrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_ = CaptureZCodePrompt([]byte(`{"session_id":"e2e-1","prompt":"hello zcode"}`))

	var gotPath string
	var gotBody struct {
		Source   string `json:"source"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	srv := newUploadStub(t, &gotPath, &gotBody)

	payload := map[string]any{
		"hook_event_name":        "Stop",
		"session_id":             "e2e-1",
		"cwd":                    "/tmp",
		"last_assistant_message": "zcode reply",
		"stop_hook_active":       false,
		"permission_mode":        "default",
	}
	raw, _ := json.Marshal(payload)

	out := runSubmit(t, "zcode", raw, srv)

	if gotPath != "/api/v1/sessions/e2e-1/upload" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody.Source != "zcode" {
		t.Fatalf("source = %q", gotBody.Source)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Content != "hello zcode" || gotBody.Messages[1].Content != "zcode reply" {
		t.Fatalf("body messages = %v", gotBody.Messages)
	}
	if !strings.Contains(out, "action=upload") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestSubmit_ZCode_SkipsNonZCodePayload(t *testing.T) {
	payload := map[string]any{
		"hook_event_name": "Stop",
		"some_field":      "some_value",
	}
	raw, _ := json.Marshal(payload)

	out := runSubmit(t, "zcode", raw, "http://127.0.0.1:1")
	if !strings.Contains(out, "action=skip") {
		t.Fatalf("expected skip, got: %q", out)
	}
	if !strings.Contains(out, "not a zcode payload") {
		t.Fatalf("expected 'not a zcode payload', got: %q", out)
	}
}

// newUploadStub starts an httptest server recording the upload path and body.
func newUploadStub(t *testing.T, gotPath *string, body any) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func runSubmit(t *testing.T, source string, payload []byte, baseURL string) string {
	t.Helper()
	var out bytes.Buffer
	if err := Submit(context.Background(), SubmitInput{
		Source:     source,
		StdinJSON:  payload,
		OutputSink: &out,
		BaseURL:    baseURL,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	return out.String()
}
