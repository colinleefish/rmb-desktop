package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/session"
)

// SubmitInput is the contract for hook-submit.
type SubmitInput struct {
	Source     string
	StdinJSON  []byte
	OutputSink io.Writer
	BaseURL    string
}

// Submit parses agent hook stdin and POSTs to rmbd upload API.
func Submit(ctx context.Context, in SubmitInput) error {
	out := in.OutputSink
	if out == nil {
		out = io.Discard
	}

	source := strings.ToLower(strings.TrimSpace(in.Source))
	if source == "" {
		return fmt.Errorf("hook-submit: --source is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if baseURL == "" {
		cfg, err := config.LoadDefault()
		if err != nil {
			return err
		}
		baseURL = cfg.BaseURL()
	}

	logf := func(action, reason string) error {
		_, err := fmt.Fprintf(out, "rmb hook-submit source=%s action=%s reason=%s target=%s\n",
			source, action, reason, baseURL)
		return err
	}

	var sessionKey string
	var messages []uploadMessage
	var reason string
	var err error

	switch source {
	case "cursor":
		if !IsCursorPayload(in.StdinJSON) {
			return logf("skip", "not a cursor payload")
		}
		sessionKey, messages, reason, err = ParseCursorPayload(in.StdinJSON)
	case "cc", "claude":
		source = "cc"
		if !IsClaudePayload(in.StdinJSON) {
			return logf("skip", "not a claude payload")
		}
		sessionKey, messages, reason, err = ParseClaudePayload(in.StdinJSON)
	default:
		return fmt.Errorf("hook-submit: unsupported source %q (cursor, cc)", source)
	}
	if err != nil {
		return logf("skip", err.Error())
	}

	body := struct {
		Source   string            `json:"source"`
		Messages []session.Message `json:"messages"`
	}{Source: source, Messages: toSessionMessages(messages)}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/sessions/%s/upload", baseURL, sessionKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("upload failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	return logf("upload", reason)
}

func toSessionMessages(msgs []uploadMessage) []session.Message {
	out := make([]session.Message, len(msgs))
	for i, m := range msgs {
		out[i] = session.Message{Role: m.Role, Content: m.Content}
	}
	return out
}
