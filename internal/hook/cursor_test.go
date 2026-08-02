package hook

import (
	"testing"
)

func TestIsCursorPayload(t *testing.T) {
	raw := []byte(`{
		"conversation_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"cursor_version": "1.0.0",
		"status": "completed",
		"text": "Here is the answer."
	}`)
	if !IsCursorPayload(raw) {
		t.Fatal("expected cursor payload")
	}
}

func TestParseCursorPayload_textFallback(t *testing.T) {
	raw := []byte(`{
		"conversation_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"cursor_version": "1.0.0",
		"status": "completed",
		"text": "kubectl apply -f deploy.yaml"
	}`)

	key, msgs, reason, err := ParseCursorPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if key != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("session key: %q", key)
	}
	if len(msgs) != 1 || msgs[0].Role != "assistant" {
		t.Fatalf("messages: %+v", msgs)
	}
	if reason != "assistant text fallback" {
		t.Fatalf("reason: %q", reason)
	}
}

func TestParseCursorPayload_skipsAborted(t *testing.T) {
	raw := []byte(`{
		"conversation_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"cursor_version": "1.0.0",
		"status": "aborted",
		"text": "partial"
	}`)
	_, _, _, err := ParseCursorPayload(raw)
	if err == nil {
		t.Fatal("expected error for aborted status")
	}
}
