package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type claudePayload struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	Cwd                  string `json:"cwd"`
	LastAssistantMessage string `json:"last_assistant_message"`
	StopHookActive       bool   `json:"stop_hook_active"`
	PermissionMode       string `json:"permission_mode"`
	HookEventName        string `json:"hook_event_name"`
}

// IsClaudePayload reports whether raw JSON looks Claude Code-originated.
func IsClaudePayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p claudePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if strings.TrimSpace(p.LastAssistantMessage) != "" {
		return true
	}
	if strings.TrimSpace(p.Cwd) != "" {
		return true
	}
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raw2); err == nil {
		if _, ok := raw2["stop_hook_active"]; ok {
			return true
		}
	}
	if tp := strings.TrimSpace(p.TranscriptPath); tp != "" {
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(tp, home+"/.claude/") {
			return true
		}
	}
	return false
}

// ParseClaudePayload extracts session key and messages from a Claude Code Stop hook.
func ParseClaudePayload(raw []byte) (sessionKey string, messages []uploadMessage, reason string, err error) {
	var p claudePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, "", fmt.Errorf("decode claude payload: %w", err)
	}

	sessionKey = strings.ToLower(strings.TrimSpace(p.SessionID))
	if sessionKey == "" {
		return "", nil, "", fmt.Errorf("claude payload missing session_id")
	}

	assistant := strings.TrimSpace(p.LastAssistantMessage)
	if assistant == "" {
		return "", nil, "", fmt.Errorf("claude payload: last_assistant_message is empty")
	}

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

type claudeTranscriptRow struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func claudeFindLastUserPrompt(transcriptPath string) string {
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
		var row claudeTranscriptRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(row.Type)) != "user" {
			continue
		}
		if text := extractRealUserText(row.Message.Content); text != "" {
			last = text
		}
	}
	return last
}

func extractRealUserText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		s := strings.TrimSpace(asString)
		if s == "" || isClaudeCommandWrapper(s) {
			return ""
		}
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var textParts []string
	for _, b := range blocks {
		t := strings.ToLower(strings.TrimSpace(b.Type))
		if t == "tool_result" {
			return ""
		}
		if t == "text" {
			if txt := strings.TrimSpace(b.Text); txt != "" {
				textParts = append(textParts, txt)
			}
		}
	}
	if len(textParts) == 0 {
		return ""
	}
	joined := strings.Join(textParts, "\n")
	if isClaudeCommandWrapper(joined) {
		return ""
	}
	return joined
}

func isClaudeCommandWrapper(s string) bool {
	if strings.HasPrefix(s, "<local-command-") {
		return true
	}
	if strings.Contains(s, "<command-name>") {
		return true
	}
	if strings.Contains(s, "<command-message>") {
		return true
	}
	if strings.Contains(s, "<local-command-stdout>") {
		return true
	}
	if strings.Contains(s, "<local-command-caveat>") {
		return true
	}
	return false
}
