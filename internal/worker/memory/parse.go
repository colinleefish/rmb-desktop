package memory

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/llm"
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
	raw, err := llm.StripCodeFence(raw)
	if err != nil {
		return ParsedMemory{}, err
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
