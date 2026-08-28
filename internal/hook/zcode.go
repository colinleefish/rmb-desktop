package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// zcodePayload covers the ZCode Stop hook payload shape. ZCode's hook stdin
// contract mirrors Claude Code's field names (session_id, transcript_path,
// cwd, permission_mode, hook_event_name, stop_hook_active,
// last_assistant_message), since ZCode inherited the same hook protocol.
//
// ZCode cleans up the transcript_path temp directory once the hook finishes,
// so it must be read synchronously while hook-submit runs (which is the
// case here: the Stop hook blocks until this process exits).
type zcodePayload struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	Cwd                  string `json:"cwd"`
	LastAssistantMessage string `json:"last_assistant_message"`
	StopHookActive       bool   `json:"stop_hook_active"`
	PermissionMode       string `json:"permission_mode"`
	HookEventName        string `json:"hook_event_name"`
}

// IsZCodePayload reports whether raw JSON looks ZCode-originated.
func IsZCodePayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p zcodePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if tp := strings.TrimSpace(p.TranscriptPath); tp != "" {
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(tp, home+"/.zcode/") {
			return true
		}
	}
	if strings.TrimSpace(p.LastAssistantMessage) != "" {
		return true
	}
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raw2); err == nil {
		if _, ok := raw2["stop_hook_active"]; ok {
			return true
		}
	}
	return false
}

// ParseZCodePayload extracts session key and messages from a ZCode Stop hook.
func ParseZCodePayload(raw []byte) (sessionKey string, messages []uploadMessage, reason string, err error) {
	var p zcodePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, "", fmt.Errorf("decode zcode payload: %w", err)
	}

	sessionKey = strings.ToLower(strings.TrimSpace(p.SessionID))
	if sessionKey == "" {
		return "", nil, "", fmt.Errorf("zcode payload missing session_id")
	}

	assistant := strings.TrimSpace(p.LastAssistantMessage)
	if assistant == "" {
		return "", nil, "", fmt.Errorf("zcode payload: last_assistant_message is empty")
	}

	// ZCode's hook transcript is Claude-Code-compatible JSONL when present;
	// reuse the same tolerant parser and degrade to assistant-only on any
	// shape mismatch (unmarshal errors are swallowed by claudeFindLastUserPrompt).
	userText := claudeFindLastUserPrompt(p.TranscriptPath)

	out := make([]uploadMessage, 0, 2)
	if userText != "" {
		out = append(out, uploadMessage{Role: "user", Content: userText})
	}
	out = append(out, uploadMessage{Role: "assistant", Content: assistant})

	if userText == "" {
		return sessionKey, out, "last_assistant_message only (no user found)", nil
	}
	return sessionKey, out, "user from transcript + assistant from payload", nil
}
