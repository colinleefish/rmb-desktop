package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
)

// Client calls the local rmbd HTTP API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a client for the given base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// FromConfig resolves the API client from rmb-desktop config.
func FromConfig(cfg config.Config) *Client {
	return New(cfg.BaseURL())
}

// Match is a search hit from the API.
type Match struct {
	URI     string  `json:"uri"`
	Tier    string  `json:"tier"`
	Rank    float64 `json:"rank"`
	Snippet string  `json:"snippet"`
}

func (c *Client) Search(ctx context.Context, query string, k int, scopes []string, since, until string) ([]Match, error) {
	q := url.Values{}
	q.Set("q", query)
	if k > 0 {
		q.Set("k", strconv.Itoa(k))
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, ","))
	}
	if since != "" {
		q.Set("since", since)
	}
	if until != "" {
		q.Set("until", until)
	}
	endpoint := c.baseURL + "/api/v1/search?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, apiError("search", resp.StatusCode, body)
	}
	var out struct {
		Items []Match `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	return out.Items, nil
}

func (c *Client) Inspect(ctx context.Context, kind, uri string) (string, error) {
	return c.InspectWith(ctx, kind, uri, nil)
}

// InspectWith is Inspect with extra query parameters (used by ls for
// --limit/--offset/--since/--until/--count).
func (c *Client) InspectWith(ctx context.Context, kind, uri string, extra url.Values) (string, error) {
	q := url.Values{}
	q.Set("uri", uri)
	for k, vs := range extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	endpoint := c.baseURL + "/api/v1/inspect/" + kind + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", kind, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", apiError("inspect/"+kind, resp.StatusCode, body)
	}
	return string(body), nil
}

// SkillFile is one file in a skill bundle upload.
type SkillFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// SkillSummary is tier-1 catalog metadata.
type SkillSummary struct {
	URI         string   `json:"uri"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Slug        string   `json:"slug"`
}

// PutSkillResult is returned after uploading a skill bundle.
type PutSkillResult struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
	NoOp    bool   `json:"no_op"`
}

func (c *Client) ListSkills(ctx context.Context) ([]SkillSummary, error) {
	endpoint := c.baseURL + "/api/v1/browse/skills?limit=500&offset=0"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, apiError("browse/skills", resp.StatusCode, body)
	}
	var out struct {
		Items []SkillSummary `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode skills response: %w", err)
	}
	return out.Items, nil
}

func (c *Client) GetSkill(ctx context.Context, slug string) (skillDetail, error) {
	endpoint := c.baseURL + "/api/v1/browse/skills/" + url.PathEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return skillDetail{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return skillDetail{}, fmt.Errorf("get skill: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return skillDetail{}, apiError("browse/skills/"+slug, resp.StatusCode, body)
	}
	var out skillDetail
	if err := json.Unmarshal(body, &out); err != nil {
		return skillDetail{}, fmt.Errorf("decode skill detail: %w", err)
	}
	return out, nil
}

type skillDetail struct {
	Skill struct {
		URI  string `json:"uri"`
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"skill"`
	Files map[string]string `json:"files"`
}

func (c *Client) PutSkill(ctx context.Context, slug string, files []SkillFile) (PutSkillResult, error) {
	payload, err := json.Marshal(map[string]any{"files": files})
	if err != nil {
		return PutSkillResult{}, err
	}
	endpoint := c.baseURL + "/api/v1/skills/" + url.PathEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return PutSkillResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PutSkillResult{}, fmt.Errorf("put skill: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return PutSkillResult{}, apiError("skills/"+slug, resp.StatusCode, body)
	}
	var out PutSkillResult
	if err := json.Unmarshal(body, &out); err != nil {
		return PutSkillResult{}, fmt.Errorf("decode put skill response: %w", err)
	}
	return out, nil
}

func apiError(path string, status int, body []byte) error {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return fmt.Errorf("%s: %s", path, e.Error)
	}
	return fmt.Errorf("%s returned %d", path, status)
}
