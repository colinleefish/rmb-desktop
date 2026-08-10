package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// codexPayload covers the Codex Stop hook payload shape.
type codexPayload struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	Cwd                  string `json:"cwd"`
	LastAssistantMessage string `json:"last_assistant_message"`
	StopHookActive       bool   `json:"stop_hook_active"`
	PermissionMode       string `json:"permission_mode"`
	HookEventName        string `json:"hook_event_name"`
	Model                string `json:"model"`
	TurnID               string `json:"turn_id"`
}

// IsCodexPayload reports whether raw JSON looks Codex-originated.
func IsCodexPayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p codexPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if strings.TrimSpace(p.Model) != "" {
		return true
	}
	if tp := strings.TrimSpace(p.TranscriptPath); tp != "" {
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(tp, home+"/.codex/") {
			return true
		}
	}
	return false
}

// ParseCodexPayload extracts session key and messages from a Codex Stop hook.
func ParseCodexPayload(raw []byte) (sessionKey string, messages []uploadMessage, reason string, err error) {
	var p codexPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, "", fmt.Errorf("decode codex payload: %w", err)
	}

	sessionKey = strings.ToLower(strings.TrimSpace(p.SessionID))
	if sessionKey == "" {
		return "", nil, "", fmt.Errorf("codex payload missing session_id")
	}

	assistant := strings.TrimSpace(p.LastAssistantMessage)
	if assistant == "" {
		return "", nil, "", fmt.Errorf("codex payload: last_assistant_message is empty")
	}

	userText := codexFindLastUserMessage(p.TranscriptPath)

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

type codexTranscriptLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"payload"`
}

func codexFindLastUserMessage(transcriptPath string) string {
	transcriptPath = strings.TrimSpace(transcriptPath)
	if transcriptPath == "" {
		return ""
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 8*1024*1024)
	last := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row codexTranscriptLine
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		if row.Type != "event_msg" || row.Payload.Type != "user_message" {
			continue
		}
		if msg := strings.TrimSpace(row.Payload.Message); msg != "" {
			last = msg
		}
	}
	return last
}
