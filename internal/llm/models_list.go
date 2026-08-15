package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxModelsErrorBodyBytes = 512
const maxModelsResponseBodyBytes = 4 << 20 // large provider catalogs can exceed 512 bytes

// OpenAI-compatible providers often mount chat under Anthropic-style subpaths.
var knownCompatSuffixes = []string{
	"/api/claudecode",
	"/api/anthropic",
	"/apps/anthropic",
	"/api/coding",
	"/claudecode",
	"/anthropic",
	"/step_plan",
	"/coding",
	"/claude",
}

type modelsListResponse struct {
	Data []modelsListItem `json:"data"`
}

type modelsListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func buildModelsURLCandidates(baseURL string) []string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return nil
	}

	var candidates []string

	if endsWithVersionSegment(trimmed) {
		candidates = append(candidates, trimmed+"/models")
		if !strings.HasSuffix(trimmed, "/v1") {
			candidates = append(candidates, trimmed+"/v1/models")
		}
	} else {
		candidates = append(candidates, trimmed+"/v1/models")
	}

	if stripped, ok := stripCompatSuffix(trimmed); ok {
		root := strings.TrimRight(stripped, "/")
		if root != "" && strings.Contains(root, "://") {
			candidates = append(candidates, root+"/v1/models", root+"/models")
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, url := range candidates {
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		unique = append(unique, url)
	}
	return unique
}

func endsWithVersionSegment(url string) bool {
	last := url
	if i := strings.LastIndex(url, "/"); i >= 0 {
		last = url[i+1:]
	}
	digits, ok := strings.CutPrefix(last, "v")
	return ok && digits != "" && strings.Trim(digits, "0123456789") == ""
}

func stripCompatSuffix(baseURL string) (string, bool) {
	for _, suffix := range knownCompatSuffixes {
		if strings.HasSuffix(baseURL, suffix) {
			return baseURL[:len(baseURL)-len(suffix)], true
		}
	}
	return "", false
}

func truncateBody(body string, max int) string {
	if len(body) <= max {
		return body
	}
	return body[:max] + "…"
}

// listModels calls OpenAI-compatible GET /models endpoints (with URL fallbacks).
// Most providers require a valid API key (Bearer).
func listModels(ctx context.Context, apiBase, apiKey string, timeout time.Duration) ([]string, error) {
	base := strings.TrimSpace(apiBase)
	key := strings.TrimSpace(apiKey)
	if base == "" {
		return nil, fmt.Errorf("llm api_base is required")
	}
	if key == "" {
		return nil, fmt.Errorf("llm api_key is required")
	}

	candidates := buildModelsURLCandidates(base)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("llm api_base is required")
	}

	client := &http.Client{Timeout: timeout}
	var lastErr error

	for _, url := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build models request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("perform models request: %w", err)
			continue
		}

		bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxModelsResponseBodyBytes))
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read models response: %w", readErr)
			continue
		}
		body := strings.TrimSpace(string(bodyBytes))

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			lastErr = fmt.Errorf("models http %d: %s", resp.StatusCode, truncateBody(body, maxModelsErrorBodyBytes))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("models http %d: %s", resp.StatusCode, truncateBody(body, maxModelsErrorBodyBytes))
		}

		if len(bodyBytes) == 0 {
			lastErr = fmt.Errorf("models http %d: empty response", resp.StatusCode)
			continue
		}

		var parsed modelsListResponse
		if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
			lastErr = fmt.Errorf("decode models response: %w", err)
			continue
		}

		ids := make([]string, 0, len(parsed.Data))
		for _, item := range parsed.Data {
			for _, id := range modelListIDs(item) {
				ids = append(ids, id)
			}
		}
		return ids, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no models endpoint candidates for %s", base)
}

func modelListIDs(item modelsListItem) []string {
	seen := make(map[string]struct{}, 2)
	var ids []string
	for _, raw := range []string{item.ID, item.Name} {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
