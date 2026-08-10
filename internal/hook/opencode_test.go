package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestIsOpenCodePayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			"agent opencode present",
			`{"agent":"opencode","session_id":"ses_abc","last_assistant_message":"hi"}`,
			true,
		},
		{
			"session_db_path under .local/share/opencode",
			`{"session_id":"ses_abc","session_db_path":"` + opencodeDBHome(t) + `/opencode.db","last_assistant_message":"hi"}`,
			true,
		},
		{
			"pi payload is not opencode",
			`{"agent":"pi","session_id":"abc","last_assistant_message":"hi"}`,
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
			if got := IsOpenCodePayload([]byte(tt.raw)); got != tt.want {
				t.Fatalf("IsOpenCodePayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenCodeRMBSessionID_DerivesUUIDFromSesPrefix(t *testing.T) {
	const opencodeID = "ses_0a53a1eafffexdrKwI0l3k9ewh"
	got, err := opencodeRMBSessionID(opencodeID)
	if err != nil {
		t.Fatalf("opencodeRMBSessionID: %v", err)
	}
	want := strings.ToLower(
		uuid.NewSHA1(opencodeSessionNamespace, []byte("opencode:"+opencodeID)).String(),
	)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("derived id is not a UUID: %v", err)
	}
}

func TestOpenCodeRMBSessionID_PassesThroughUUID(t *testing.T) {
	const raw = "4F1916CE-2F6E-4B76-8249-4A5F4184FD8D"
	got, err := opencodeRMBSessionID(raw)
	if err != nil {
		t.Fatalf("opencodeRMBSessionID: %v", err)
	}
	if got != "4f1916ce-2f6e-4b76-8249-4a5f4184fd8d" {
		t.Fatalf("got %q", got)
	}
}

func TestParseOpenCodePayload_WithUserAndAssistant(t *testing.T) {
	payload := map[string]any{
		"agent":                  "opencode",
		"session_id":             "ses_test123",
		"last_user_message":      "what is opencode?",
		"last_assistant_message": "OpenCode is a terminal coding agent.",
		"hook_event_name":        "session.status",
	}
	raw, _ := json.Marshal(payload)

	sid, msgs, reason, err := ParseOpenCodePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSID, err := opencodeRMBSessionID("ses_test123")
	if err != nil {
		t.Fatalf("opencodeRMBSessionID: %v", err)
	}
	if sid != wantSID {
		t.Fatalf("session_id = %q, want %q", sid, wantSID)
	}
	if reason != "user and assistant from payload" {
		t.Fatalf("reason = %q", reason)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2; msgs = %v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "what is opencode?" {
		t.Fatalf("msgs[0] = %#v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "OpenCode is a terminal coding agent." {
		t.Fatalf("msgs[1] = %#v", msgs[1])
	}
}

func TestSubmit_OpenCode_UploadsToAPI(t *testing.T) {
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
		"agent":                  "opencode",
		"session_id":             "ses_upload_test",
		"last_user_message":      "hello opencode",
		"last_assistant_message": "opencode reply",
		"hook_event_name":        "session.idle",
	}
	raw, _ := json.Marshal(payload)

	wantPathID, err := opencodeRMBSessionID("ses_upload_test")
	if err != nil {
		t.Fatalf("opencodeRMBSessionID: %v", err)
	}

	var out bytes.Buffer
	if err := Submit(context.Background(), SubmitInput{
		Source:     "opencode",
		StdinJSON:  raw,
		OutputSink: &out,
		BaseURL:    srv.URL,
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if gotPath != "/api/v1/sessions/"+wantPathID+"/upload" {
		t.Fatalf("path = %q, want /api/v1/sessions/%s/upload", gotPath, wantPathID)
	}
	if gotBody.Source != "opencode" {
		t.Fatalf("source = %q", gotBody.Source)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Content != "hello opencode" || gotBody.Messages[1].Content != "opencode reply" {
		t.Fatalf("body messages = %v", gotBody.Messages)
	}
	if !strings.Contains(out.String(), "action=upload") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func opencodeDBHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return home + "/.local/share/opencode"
}
