package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

// zcodeSessionNamespace maps non-reducible ZCode session ids to deterministic
// rmb UUIDs (uuid5), mirroring opencodeSessionNamespace in opencode.go.
var zcodeSessionNamespace = uuid.MustParse("9f3c7b2a-5d41-4e68-a7c9-2b4d6e8f1a3c")

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

	sessionKey, err = zcodeRMBSessionID(p.SessionID)
	if err != nil {
		return "", nil, "", err
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

// zcodeRMBSessionID normalizes ZCode's native session ids to the bare-UUID
// form every other agent stores (issue #62). ZCode uses sess_<uuid> for
// interactive sessions and sess_subagent_agent_<uuid> for subagent children.
//
// The function is total: valid UUID out for ANY non-empty input. Bare UUIDs
// pass through canonicalized; a sess_ prefix is stripped only when the
// remainder is a valid UUID (a pure strip is NOT total — the subagent form
// does not reduce to a UUID); anything else derives a deterministic uuid5,
// so the same native id always maps to the same stored session.
func zcodeRMBSessionID(raw string) (string, error) {
	sessionID := strings.TrimSpace(raw)
	if sessionID == "" {
		return "", fmt.Errorf("zcode payload missing session_id")
	}
	if parsed, err := uuid.Parse(sessionID); err == nil {
		return strings.ToLower(parsed.String()), nil
	}
	if rest, ok := strings.CutPrefix(strings.ToLower(sessionID), "sess_"); ok {
		if parsed, err := uuid.Parse(rest); err == nil {
			return strings.ToLower(parsed.String()), nil
		}
	}
	derived := uuid.NewSHA1(zcodeSessionNamespace, []byte("zcode:"+sessionID))
	return strings.ToLower(derived.String()), nil
}
