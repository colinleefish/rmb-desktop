package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

type llmMemoryResponse struct {
	Abstract string `json:"abstract"`
	Body     string `json:"body"`
}

type ParsedMemory struct {
	Abstract string
	Body     string
}

func parseDistillResponse(raw string) (ParsedMemory, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedMemory{}, fmt.Errorf("empty llm response")
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

	var resp llmMemoryResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return ParsedMemory{}, fmt.Errorf("decode memory json: %w", err)
	}
	abstract := strings.TrimSpace(resp.Abstract)
	body := strings.TrimSpace(resp.Body)
	if body == "" {
		return ParsedMemory{}, fmt.Errorf("memory body is empty")
	}
	if abstract == "" {
		abstract = body
	}
	return ParsedMemory{Abstract: abstract, Body: body}, nil
}
