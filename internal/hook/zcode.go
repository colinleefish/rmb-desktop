package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ZCode hook contract (verified against ZCode's CLI bundle, glm/zcode.cjs —
// createClaudeCompatibleHookStdin / formatClaudeTranscript):
//
//   - Stop payload: {session_id, hook_event_name, permission_mode,
//     transcript_path, last_assistant_message, stop_hook_active, cwd, mode,
//     responseText, responsePreview, stopHookActive, toolCallCount, turnId,
//     traceId, timestamp, agentName, agent_type}. It carries NO user prompt.
//   - The Stop transcript_path points at a temp file
//     ($TMPDIR/zcode-claude-hook-*/transcript.jsonl) holding exactly ONE line
//     — the assistant message, in a Claude-ish shape without a row "type":
//     {"message":{"content":[{"text":...,"type":"text"}],"role":"assistant"}}
//     The temp dir is deleted once the hook finishes.
//
// So the user prompt cannot come from the Stop payload or its transcript.
// ZCode's UserPromptSubmit hook does receive {prompt, session_id, ...}; we
// register a second capture hook on that event (see internal/setup/zcode.go
// and rmb hook-capture) which persists the prompt to a sidecar file keyed by
// session id. ParseZCodePayload pairs the captured prompt with the assistant
// message and consumes the sidecar.
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
// Note: the transcript_path lives under the system temp dir, not ~/.zcode, so
// detection leans on the Claude-alias fields ZCode always includes.
//
// TC-4 decision (investigation §4): a payload carrying only the camelCase
// aliases (responseText, no snake-case last_assistant_message) is NOT
// accepted. ZCode always sends both spellings (§1.2 Fact C), so camel-only
// would mean a client contract change; gating on the snake alias keeps
// detection strict — a contract change surfaces as a loud "not a zcode
// payload" skip instead of a silent misparse.
func IsZCodePayload(raw []byte) bool {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false
	}
	var p zcodePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return false
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
// The user prompt comes from the sidecar written by the UserPromptSubmit
// capture hook; when absent (capture hook not applied yet, or rmb restarted
// mid-turn) the turn degrades to assistant-only.
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

	userText := takeCapturedZCodePrompt(sessionKey)

	out := make([]uploadMessage, 0, 2)
	if userText != "" {
		out = append(out, uploadMessage{Role: "user", Content: userText})
	}
	out = append(out, uploadMessage{Role: "assistant", Content: assistant})

	if userText == "" {
		return sessionKey, out, "last_assistant_message only (no captured prompt)", nil
	}
	return sessionKey, out, "user from prompt capture + assistant from payload", nil
}

// zcodePromptPayload covers the UserPromptSubmit hook stdin: it carries the
// raw prompt plus the session id the prompt belongs to.
type zcodePromptPayload struct {
	SessionID     string `json:"session_id"`
	Prompt        string `json:"prompt"`
	HookEventName string `json:"hook_event_name"`
}

// zcodePromptDir is where UserPromptSubmit captures are parked until the
// matching Stop hook consumes them.
func zcodePromptDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".rmb", "cache", "agent-prompts", "zcode")
}

func zcodePromptPath(sessionKey string) string {
	return filepath.Join(zcodePromptDir(), sessionKey+".json")
}

// zcodeSidecarKey normalizes a payload session id into a sidecar file key.
// Same derivation as ParseZCodePayload's session key (lowercased, trimmed —
// issue #62 owns any further normalization), plus a guard: the key must form
// a single safe path segment so a hostile session_id cannot escape the
// sidecar dir. Returns "" when unusable.
func zcodeSidecarKey(sessionID string) string {
	key := strings.ToLower(strings.TrimSpace(sessionID))
	if key == "" || strings.ContainsAny(key, `/\`) {
		return ""
	}
	return key
}

// CaptureZCodePrompt persists a UserPromptSubmit prompt for later pairing by
// ParseZCodePayload. It never fails the hook: ZCode treats non-zero exits as
// run failures, and a missed capture only degrades to assistant-only capture.
func CaptureZCodePrompt(raw []byte) error {
	var p zcodePromptPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil //nolint:nilerr // undecorable stdin: skip silently
	}
	sessionKey := zcodeSidecarKey(p.SessionID)
	prompt := strings.TrimSpace(p.Prompt)
	if sessionKey == "" || prompt == "" {
		return nil
	}

	dir := zcodePromptDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}

	doc := struct {
		Prompt     string `json:"prompt"`
		CapturedAt int64  `json:"captured_at"`
	}{Prompt: prompt, CapturedAt: time.Now().Unix()}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil
	}

	// Atomic write: last prompt wins if several are queued before a Stop.
	tmp := zcodePromptPath(sessionKey) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return nil
	}
	_ = os.Rename(tmp, zcodePromptPath(sessionKey))

	zcodeSweepStalePrompts(dir)
	return nil
}

// takeCapturedZCodePrompt returns and removes the prompt captured for a
// session, or "" when none exists.
func takeCapturedZCodePrompt(sessionKey string) string {
	key := zcodeSidecarKey(sessionKey)
	if key == "" {
		return ""
	}
	path := zcodePromptPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	_ = os.Remove(path)

	var doc struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Prompt)
}

// zcodeSweepStalePrompts best-effort deletes captures that were never
// consumed (session abandoned before a Stop hook ran).
func zcodeSweepStalePrompts(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
