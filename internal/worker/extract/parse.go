package extract

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
)

type llmAtom struct {
	Category          string `json:"category"`
	Priority          int    `json:"priority"`
	SceneName         string `json:"scene_name"`
	Slug              string `json:"slug"`
	Content           string `json:"content"`
	SourceTurnIndices []int  `json:"source_turn_indices"`
}

type llmExtractResponse struct {
	Atoms []llmAtom `json:"atoms"`
}

func parseExtractResponse(raw string) ([]llmAtom, error) {
	raw, err := llm.StripCodeFence(raw)
	if err != nil {
		return nil, err
	}

	var resp llmExtractResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("decode atoms json: %w", err)
	}
	if len(resp.Atoms) == 0 {
		return nil, nil
	}

	out := make([]llmAtom, 0, len(resp.Atoms))
	for i, a := range resp.Atoms {
		cat := strings.ToLower(strings.TrimSpace(a.Category))
		if _, ok := model.ValidAtomCategories[cat]; !ok {
			return nil, fmt.Errorf("atoms[%d]: invalid category %q", i, a.Category)
		}
		content := strings.TrimSpace(a.Content)
		if content == "" {
			continue
		}
		a.Category = cat
		a.Content = content
		out = append(out, a)
	}
	return out, nil
}
