package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

// opencodeSessionNamespace maps OpenCode ses_* IDs to deterministic rmb UUIDs.
var opencodeSessionNamespace = uuid.MustParse("c4e8f1a2-6b3d-4f9e-a1c2-8d7e6f5a4b3c")

type opencodePayload struct {
	Agent                string `json:"agent"`
	SessionID            string `json:"session_id"`
	LastUserMessage      string `json:"last_user_message"`
	LastAssistantMessage string `json:"last_assistant_message"`
	SessionDBPath        string `json:"session_db_path"`
	Cwd                  string `json:"cwd"`
	HookEventName        string `json:"hook_event_name"`
}

// IsOpenCodePayload reports whether raw JSON looks OpenCode-originated.
func IsOpenCodePayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p opencodePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(p.Agent), "opencode") {
		return true
	}
	return opencodeDBPath(p.SessionDBPath)
}

func opencodeDBPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	for _, suffix := range []string{
		"/.local/share/opencode/opencode.db",
		"/Library/Application Support/opencode/opencode.db",
	} {
		if path == home+suffix {
			return true
		}
	}
	return false
}

// ParseOpenCodePayload extracts session key and messages from an OpenCode plugin payload.
func ParseOpenCodePayload(raw []byte) (sessionKey string, messages []uploadMessage, reason string, err error) {
	var p opencodePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, "", fmt.Errorf("decode opencode payload: %w", err)
	}

	sessionKey, err = opencodeRMBSessionID(p.SessionID)
	if err != nil {
		return "", nil, "", err
	}

	assistant := strings.TrimSpace(p.LastAssistantMessage)
	if assistant == "" {
		return "", nil, "", fmt.Errorf("opencode payload: last_assistant_message is empty")
	}

	userText := strings.TrimSpace(p.LastUserMessage)

	out := make([]uploadMessage, 0, 2)
	if userText != "" {
		out = append(out, uploadMessage{Role: "user", Content: userText})
	}
	out = append(out, uploadMessage{Role: "assistant", Content: assistant})

	if userText == "" {
		return sessionKey, out, "last_assistant_message only (no user found)", nil
	}
	return sessionKey, out, "user and assistant from payload", nil
}

func opencodeRMBSessionID(raw string) (string, error) {
	sessionID := strings.TrimSpace(raw)
	if sessionID == "" {
		return "", fmt.Errorf("opencode payload missing session_id")
	}
	if parsed, err := uuid.Parse(sessionID); err == nil {
		return strings.ToLower(parsed.String()), nil
	}
	derived := uuid.NewSHA1(opencodeSessionNamespace, []byte("opencode:"+sessionID))
	return strings.ToLower(derived.String()), nil
}
