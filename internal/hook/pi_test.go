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

func TestIsPiPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			"agent pi present",
			`{"agent":"pi","session_id":"abc","last_assistant_message":"hi"}`,
			true,
		},
		{
			"session_file under .pi/agent",
			`{"session_id":"abc","session_file":"` + piHome(t) + `/sessions/test.jsonl"}`,
			true,
		},
		{
			"transcript_path under .pi/agent",
			`{"session_id":"abc","transcript_path":"` + piHome(t) + `/sessions/test.jsonl"}`,
			true,
		},
		{
			"codex payload is not pi",
			`{"session_id":"abc","model":"gpt-5","last_assistant_message":"hi"}`,
			false,
		},
		{
			"empty payload",
			`{}`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPiPayload([]byte(tt.raw)); got != tt.want {
				t.Fatalf("IsPiPayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePiPayload_WithUserAndAssistant(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	rawTranscript := strings.Join([]string{
		`{"type":"session","version":3,"id":"sess-uuid","timestamp":"2024-12-03T14:00:00.000Z","cwd":"/proj"}`,
		`{"type":"message","id":"a1","parentId":null,"timestamp":"2024-12-03T14:00:01.000Z","message":{"role":"user","content":"what is pi?"}}`,
		`{"type":"message","id":"a2","parentId":"a1","timestamp":"2024-12-03T14:00:02.000Z","message":{"role":"assistant","content":[{"type":"text","text":"Pi is a minimal agent harness."}]}}`,
		`{"type":"message","id":"a3","parentId":"a2","timestamp":"2024-12-03T14:00:03.000Z","message":{"role":"user","content":[{"type":"text","text":"tell me more"}]}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(rawTranscript), 0o600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	payload := map[string]any{
		"agent":                  "pi",
		"session_id":             "sess-uuid",
		"session_file":           transcriptPath,
		"cwd":                    "/proj",
		"last_assistant_message": "Pi ships with extensions and skills.",
		"hook_event_name":        "agent_settled",
	}
	raw, _ := json.Marshal(payload)

	sid, msgs, reason, err := ParsePiPayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sid != "sess-uuid" {
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
	if msgs[1].Role != "assistant" || msgs[1].Content != "Pi ships with extensions and skills." {
		t.Fatalf("msgs[1] = %#v", msgs[1])
	}
}

func TestSubmit_Pi_UploadsToAPI(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "session.jsonl")
	rawTranscript := strings.Join([]string{
		`{"type":"message","id":"a1","parentId":null,"timestamp":"2024-12-03T14:00:01.000Z","message":{"role":"user","content":"hello pi"}}`,
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
		"agent":                  "pi",
		"session_id":             "pi-session-123",
		"session_file":           transcriptPath,
		"last_assistant_message": "pi reply",
		"hook_event_name":        "agent_settled",
	}
	raw, _ := json.Marshal(payload)

	var out bytes.Buffer
	if err := Submit(context.Background(), SubmitInput{
		Source:     "pi",
		StdinJSON:  raw,
		OutputSink: &out,
		BaseURL:    srv.URL,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if gotPath != "/api/v1/sessions/pi-session-123/upload" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody.Source != "pi" {
		t.Fatalf("source = %q", gotBody.Source)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Content != "hello pi" || gotBody.Messages[1].Content != "pi reply" {
		t.Fatalf("body messages = %v", gotBody.Messages)
	}
	if !strings.Contains(out.String(), "action=upload") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func piHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return home + "/.pi/agent"
}
