package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type uploadMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cursorPayload struct {
	ConversationID string   `json:"conversation_id"`
	SessionID      string   `json:"session_id"`
	Text           string   `json:"text"`
	TranscriptPath string   `json:"transcript_path"`
	CursorVersion  string   `json:"cursor_version"`
	WorkspaceRoots []string `json:"workspace_roots"`
	Status         string   `json:"status"`
}

// IsCursorPayload reports whether raw JSON looks Cursor-originated.
func IsCursorPayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
	}
	if strings.TrimSpace(p.CursorVersion) != "" {
		return true
	}
	if len(p.WorkspaceRoots) > 0 {
		return true
	}
	if tp := strings.TrimSpace(p.TranscriptPath); tp != "" {
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(tp, home+"/.cursor/") {
			return true
		}
	}
	return false
}

// ParseCursorPayload extracts session key and messages from a Cursor hook payload.
func ParseCursorPayload(raw []byte) (sessionKey string, messages []uploadMessage, reason string, err error) {
	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, "", fmt.Errorf("decode cursor payload: %w", err)
	}

	if status := strings.ToLower(strings.TrimSpace(p.Status)); status != "" && status != "completed" {
		return "", nil, "", fmt.Errorf("cursor payload status=%q is not completed", p.Status)
	}

	sessionKey = strings.ToLower(strings.TrimSpace(p.ConversationID))
	if sessionKey == "" {
		sessionKey = strings.ToLower(strings.TrimSpace(p.SessionID))
	}
	if sessionKey == "" {
		return "", nil, "", fmt.Errorf("cursor payload missing conversation/session id")
	}

	if msgs := buildPairFromTranscript(p.TranscriptPath, p.Text); len(msgs) > 0 {
		return sessionKey, msgs, "latest user/assistant from transcript", nil
	}

	assistant := strings.TrimSpace(p.Text)
	if assistant == "" {
		return "", nil, "", fmt.Errorf("cursor payload: no messages extracted and text is empty")
	}
	return sessionKey, []uploadMessage{{Role: "assistant", Content: assistant}}, "assistant text fallback", nil
}

type transcriptRow struct {
	Role    string `json:"role"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

type textMessage struct {
	Role string
	Text string
}

func buildPairFromTranscript(transcriptPath, assistantText string) []uploadMessage {
	msgs := readTranscript(transcriptPath)
	if len(msgs) == 0 {
		return nil
	}

	assistantText = strings.TrimSpace(assistantText)
	assistantIdx := -1

	if assistantText != "" {
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "assistant" && strings.TrimSpace(msgs[i].Text) == assistantText {
				assistantIdx = i
				break
			}
		}
	}
	if assistantIdx < 0 {
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "assistant" {
				assistantIdx = i
				if assistantText == "" {
					assistantText = msgs[i].Text
				}
				break
			}
		}
	}
	if assistantIdx < 0 || strings.TrimSpace(assistantText) == "" {
		return nil
	}

	userText := ""
	for i := assistantIdx - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			userText = msgs[i].Text
			break
		}
	}

	out := make([]uploadMessage, 0, 2)
	if strings.TrimSpace(userText) != "" {
		out = append(out, uploadMessage{Role: "user", Content: userText})
	}
	out = append(out, uploadMessage{Role: "assistant", Content: assistantText})
	return out
}

func readTranscript(transcriptPath string) []textMessage {
	transcriptPath = strings.TrimSpace(transcriptPath)
	if transcriptPath == "" {
		return nil
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []textMessage
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var row transcriptRow
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			continue
		}
		role := strings.TrimSpace(row.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		var parts []string
		for _, block := range row.Message.Content {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				parts = append(parts, block.Text)
			}
		}
		text := strings.TrimSpace(strings.Join(parts, "\n"))
		if text == "" {
			continue
		}
		out = append(out, textMessage{Role: role, Text: text})
	}
	return out
}
