package llm

import (
	"errors"
	"strings"
)

// StripCodeFence normalizes a raw LLM response for JSON decoding: it trims
// surrounding whitespace and unwraps a single Markdown code fence
// (```lang ... ```), returning the payload. Empty input is an error.
func StripCodeFence(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty llm response")
	}
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) >= 2 {
			end := len(lines)
			if strings.TrimSpace(lines[end-1]) == "```" {
				end--
			}
			raw = strings.Join(lines[1:end], "\n")
		}
	}
	return raw, nil
}
