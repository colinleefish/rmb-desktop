package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type piPayload struct {
	Agent                string `json:"agent"`
	SessionID            string `json:"session_id"`
	SessionFile          string `json:"session_file"`
	TranscriptPath       string `json:"transcript_path"`
	Cwd                  string `json:"cwd"`
	LastAssistantMessage string `json:"last_assistant_message"`
	HookEventName        string `json:"hook_event_name"`
}

// IsPiPayload reports whether raw JSON looks Pi-originated.
func IsPiPayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p piPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(p.Agent), "pi") {
		return true
	}
	for _, path := range []string{p.SessionFile, p.TranscriptPath} {
		if piSessionPath(path) {
			return true
		}
	}
	return false
}

func piSessionPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	return strings.HasPrefix(path, home+"/.pi/agent/")
}

// ParsePiPayload extracts session key and messages from a Pi extension payload.
func ParsePiPayload(raw []byte) (sessionKey string, messages []uploadMessage, reason string, err error) {
	var p piPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, "", fmt.Errorf("decode pi payload: %w", err)
	}

	sessionKey = strings.ToLower(strings.TrimSpace(p.SessionID))
	if sessionKey == "" {
		return "", nil, "", fmt.Errorf("pi payload missing session_id")
	}

	assistant := strings.TrimSpace(p.LastAssistantMessage)
	if assistant == "" {
		return "", nil, "", fmt.Errorf("pi payload: last_assistant_message is empty")
	}

	transcriptPath := strings.TrimSpace(p.SessionFile)
	if transcriptPath == "" {
		transcriptPath = strings.TrimSpace(p.TranscriptPath)
	}
	userText := piFindLastUserMessage(transcriptPath)

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

type piSessionLine struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func piFindLastUserMessage(transcriptPath string) string {
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
		var row piSessionLine
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		if row.Type != "message" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(row.Message.Role)) != "user" {
			continue
		}
		if text := piExtractUserText(row.Message.Content); text != "" {
			last = text
		}
	}
	return last
}

func piExtractUserText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
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
		if strings.ToLower(strings.TrimSpace(b.Type)) != "text" {
			continue
		}
		if t := strings.TrimSpace(b.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
}
