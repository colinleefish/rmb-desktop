package scene

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/uri"
)

const defaultSceneName = "General"

type llmScene struct {
	DisplayName string   `json:"display_name"`
	Abstract    string   `json:"abstract"`
	Body        string   `json:"body"`
	AtomURIs    []string `json:"atom_uris"`
}

type llmBuildScenesResponse struct {
	Scenes []llmScene `json:"scenes"`
}

type ParsedScene struct {
	DisplayName string
	Abstract    string
	Body        string
	SourceAtoms []string
}

func parseBuildScenesResponse(raw string, validURIs map[string]struct{}) ([]ParsedScene, error) {
	raw, err := llm.StripCodeFence(raw)
	if err != nil {
		return nil, err
	}

	var resp llmBuildScenesResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("decode scenes json: %w", err)
	}
	if len(resp.Scenes) == 0 {
		return nil, fmt.Errorf("no scenes in llm response")
	}

	out := make([]ParsedScene, 0, len(resp.Scenes))
	for _, s := range resp.Scenes {
		abstract := strings.TrimSpace(s.Abstract)
		body := strings.TrimSpace(s.Body)
		if abstract == "" || body == "" {
			continue
		}
		displayName := strings.TrimSpace(s.DisplayName)
		if displayName == "" {
			displayName = defaultSceneName
		}

		atomIDs := make([]string, 0, len(s.AtomURIs))
		seen := make(map[string]struct{})
		for _, u := range s.AtomURIs {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if _, ok := validURIs[u]; !ok {
				continue
			}
			id, err := uri.ParseAtomID(u)
			if err != nil {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			atomIDs = append(atomIDs, id)
		}
		if len(atomIDs) == 0 {
			continue
		}
		out = append(out, ParsedScene{
			DisplayName: displayName,
			Abstract:    abstract,
			Body:        body,
			SourceAtoms: atomIDs,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no usable scenes in llm response")
	}
	return out, nil
}

func joinSceneAbstracts(scenes []ParsedScene) string {
	var b strings.Builder
	for i, s := range scenes {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("- ")
		b.WriteString(s.DisplayName)
		b.WriteString(": ")
		b.WriteString(s.Abstract)
	}
	return b.String()
}
