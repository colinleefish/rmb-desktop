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

	"github.com/google/uuid"
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

// TestZcodeRMBSessionID covers the session-key normalization for issue #62
// (INVESTIGATION §4 TC-5, TC-6): bare UUIDs pass through canonicalized, the
// sess_ prefix is stripped when the remainder is a valid UUID, and any other
// id gets a deterministic uuid5 derive (total function).
func TestZcodeRMBSessionID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"bare uuid passthrough",
			"f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
			"f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
		},
		{
			"uppercase uuid canonicalized",
			"F9C1B16F-B6D1-4BD3-ADB0-D0212AF43158",
			"f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
		},
		{
			"sess_ prefix stripped (interactive session id)",
			"sess_f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
			"f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
		},
		{
			"uppercase sess_ prefix stripped",
			"SESS_F9C1B16F-B6D1-4BD3-ADB0-D0212AF43158",
			"f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
		},
		{
			"subagent id falls back to deterministic uuid5",
			"sess_subagent_agent_a6f1b071-e073-411c-81d4-04f787791151",
			"d7d0e347-8296-59f1-8ea7-fcadec2dbacb",
		},
		{
			"garbage input falls back to deterministic uuid5",
			"abc-123",
			"93823071-8a64-5a8c-9c7b-381ecffa23ba",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := zcodeRMBSessionID(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("zcodeRMBSessionID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	for _, in := range []string{"", "   ", "\t\n"} {
		if _, err := zcodeRMBSessionID(in); err == nil || !strings.Contains(err.Error(), "missing session_id") {
			t.Fatalf("zcodeRMBSessionID(%q) error = %v, want missing session_id", in, err)
		}
	}
}

// TestZcodeRMBSessionID_Deterministic locks in TC-6's totality contract:
// the same native id always maps to the same stored session, and different
// ids never collide (uuid5 derive is a pure function of the input).
func TestZcodeRMBSessionID_Deterministic(t *testing.T) {
	inputs := []string{
		"sess_subagent_agent_a6f1b071-e073-411c-81d4-04f787791151",
		"sess_subagent_agent_f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
		"abc-123",
	}
	for _, in := range inputs {
		first, err := zcodeRMBSessionID(in)
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", in, err)
		}
		second, err := zcodeRMBSessionID(in)
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", in, err)
		}
		if first != second {
			t.Fatalf("%q: not deterministic: %q vs %q", in, first, second)
		}
		if _, err := uuid.Parse(first); err != nil {
			t.Fatalf("%q: derived key %q is not a valid UUID: %v", in, first, err)
		}
	}
	a, _ := zcodeRMBSessionID(inputs[0])
	b, _ := zcodeRMBSessionID(inputs[1])
	if a == b {
		t.Fatalf("distinct inputs derived the same key %q", a)
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
		"session_id":             "Sess_A2FBEA3F-E073-411C-81D4-04F787791151",
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
	if sid != "a2fbea3f-e073-411c-81d4-04f787791151" {
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
		"session_id":             "sess_f9c1b16f-b6d1-4bd3-adb0-d0212af43158",
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

	if gotPath != "/api/v1/sessions/f9c1b16f-b6d1-4bd3-adb0-d0212af43158/upload" {
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
