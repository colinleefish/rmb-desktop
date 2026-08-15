package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// workbuddyPayload covers the WorkBuddy (Tencent CodeBuddy) Stop hook payload.
// WorkBuddy passes a JSON object on stdin with the same field names used by
// Claude Code and Codex, so detection leans on the transcript path location
// (~/.workbuddy/...) rather than field presence alone.
type workbuddyPayload struct {
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	Cwd                  string `json:"cwd"`
	LastAssistantMessage string `json:"last_assistant_message"`
	StopHookActive       bool   `json:"stop_hook_active"`
	PermissionMode       string `json:"permission_mode"`
	HookEventName        string `json:"hook_event_name"`
}

// IsWorkBuddyPayload reports whether raw JSON looks WorkBuddy-originated.
func IsWorkBuddyPayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p workbuddyPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if tp := strings.TrimSpace(p.TranscriptPath); tp != "" {
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(tp, home+"/.workbuddy/") {
			return true
		}
		// A transcript under another agent's directory is not WorkBuddy.
		return false
	}
	// No transcript path: accept only a bare Stop event that is not codex
	// (codex always carries model/turn_id) and not claude (claude carries
	// last_assistant_message). WorkBuddy omits last_assistant_message unless
	// the session provides one.
	if !strings.EqualFold(strings.TrimSpace(p.HookEventName), "stop") {
		return false
	}
	if strings.TrimSpace(p.SessionID) == "" {
		return false
	}
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raw2); err != nil {
		return false
	}
	if _, hasModel := raw2["model"]; hasModel {
		return false
	}
	if _, hasTurn := raw2["turn_id"]; hasTurn {
		return false
	}
	if _, hasLast := raw2["last_assistant_message"]; hasLast {
		return false
	}
	return true
}

// ParseWorkBuddyPayload extracts session key and messages from a WorkBuddy Stop hook.
//
// WorkBuddy only includes last_assistant_message when the session provides one,
// so the assistant text falls back to the transcript when the payload omits it.
func ParseWorkBuddyPayload(raw []byte) (sessionKey string, messages []uploadMessage, reason string, err error) {
	var p workbuddyPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, "", fmt.Errorf("decode workbuddy payload: %w", err)
	}

	sessionKey = strings.ToLower(strings.TrimSpace(p.SessionID))
	if sessionKey == "" {
		return "", nil, "", fmt.Errorf("workbuddy payload missing session_id")
	}

	userText, assistantText := workbuddyFindLastExchange(p.TranscriptPath)
	if strings.TrimSpace(assistantText) == "" {
		assistantText = strings.TrimSpace(p.LastAssistantMessage)
	}
	if assistantText == "" {
		return "", nil, "", fmt.Errorf("workbuddy payload: no assistant message in payload or transcript")
	}

	out := make([]uploadMessage, 0, 2)
	if userText != "" {
		out = append(out, uploadMessage{Role: "user", Content: userText})
	}
	out = append(out, uploadMessage{Role: "assistant", Content: assistantText})

	if userText == "" {
		return sessionKey, out, "assistant from transcript/payload only (no user found)", nil
	}
	return sessionKey, out, "user + assistant from transcript", nil
}

type workbuddyTranscriptRow struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// workbuddyFindLastExchange scans the WorkBuddy session JSONL for the last
// real user message and the last assistant message. WorkBuddy injects system
// reminders (identity files, OS context) into user turns, which are skipped.
func workbuddyFindLastExchange(transcriptPath string) (userText, assistantText string) {
	transcriptPath = strings.TrimSpace(transcriptPath)
	if transcriptPath == "" {
		return "", ""
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row workbuddyTranscriptRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(row.Type)) != "message" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(row.Role)) {
		case "user":
			if text := workbuddyExtractContent(row.Content, "input_text"); text != "" {
				userText = text
			}
		case "assistant":
			if text := workbuddyExtractContent(row.Content, "output_text"); text != "" {
				assistantText = text
			}
		}
	}
	return userText, assistantText
}

// workbuddyExtractContent joins text blocks of the given type from a WorkBuddy
// content array. User turns are skipped when they only carry injected context
// (system reminders, identity files); a real prompt embedded as
// <user_query>...</user_query> inside such a block is extracted instead.
func workbuddyExtractContent(raw json.RawMessage, blockType string) string {
	if len(raw) == 0 {
		return ""
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if strings.ToLower(strings.TrimSpace(b.Type)) != blockType {
			continue
		}
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		if blockType == "input_text" {
			if q := workbuddyUserQuery(text); q != "" {
				parts = append(parts, q)
				continue
			}
			if workbuddyIsInjectedContext(text) {
				continue
			}
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return ""
	}
	joined := strings.TrimSpace(strings.Join(parts, "\n"))
	if blockType == "input_text" && workbuddyIsInjectedContext(joined) {
		return ""
	}
	return joined
}

// workbuddyUserQuery extracts the real prompt from a WorkBuddy user turn.
// WorkBuddy appends the user's message as <user_query>...</user_query> after
// the injected system reminder in the same content block.
func workbuddyUserQuery(s string) string {
	start := strings.Index(s, "<user_query>")
	if start < 0 {
		return ""
	}
	end := strings.Index(s, "</user_query>")
	if end < 0 || end <= start+len("<user_query>") {
		return ""
	}
	q := strings.TrimSpace(s[start+len("<user_query>") : end])
	if q == "" {
		return ""
	}
	return q
}

// workbuddyIsInjectedContext reports whether a user-turn text is WorkBuddy
// machinery (system reminders, command output, file snapshots) rather than a
// real prompt from the user.
func workbuddyIsInjectedContext(s string) bool {
	if strings.HasPrefix(s, "<system-reminder") {
		return true
	}
	if strings.Contains(s, "<user_info>") && strings.Contains(s, "<identity_context>") {
		return true
	}
	return false
}
