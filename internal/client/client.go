package client

import (
	"bytes"
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
	Version int     `json:"version,omitempty"`
}

func (c *Client) Search(ctx context.Context, query string, k int, scopes []string, since, until string, noBoost bool) ([]Match, error) {
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
	if noBoost {
		q.Set("no_boost", "1")
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

// ArchiveCandidate is one cold memory proposed for archival (issue #32).
type ArchiveCandidate struct {
	URI       string  `json:"uri"`
	Category  string  `json:"category"`
	Slug      string  `json:"slug,omitempty"`
	Abstract  string  `json:"abstract,omitempty"`
	Version   int     `json:"version"`
	Heat      float64 `json:"heat"`
	LastUseAt *int64  `json:"last_use_at,omitempty"`
	UpdatedAt int64   `json:"updated_at"`
}

// DoctorArchiveCandidates fetches the doctor's proposed archive list
// (read-only review / --dry-run). days <= 0 uses the 90-day default.
func (c *Client) DoctorArchiveCandidates(ctx context.Context, days int) ([]ArchiveCandidate, error) {
	endpoint := c.baseURL + "/api/v1/doctor/archive"
	if days > 0 {
		endpoint += "?days=" + strconv.Itoa(days)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("archive candidates request: %w", err)
	}
	defer resp.Body.Close()
	// The proposal can be large (thousands of cold memories), so allow a
	// generous read cap rather than the default 1 MiB used by other methods.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, apiError("doctor archive", resp.StatusCode, body)
	}
	var out struct {
		Candidates []ArchiveCandidate `json:"candidates"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode archive candidates: %w", err)
	}
	return out.Candidates, nil
}

// DoctorMetrics reports the retrieval-health signals behind the doctor
// command (issue #24): zero-cat search rate and heat concentration.
type DoctorMetrics struct {
	WindowDays        int     `json:"window_days"`
	Searches          int64   `json:"searches"`
	ConvertedSearch   int64   `json:"converted_searches"`
	ZeroCatRate       float64 `json:"zero_cat_search_rate"`
	TotalCats         int64   `json:"total_cats"`
	TopCats           int64   `json:"top_heats_cats"`
	HeatConcentration float64 `json:"heat_concentration"`
	HeatAlarm         bool    `json:"heat_concentration_alarm"`
}

// DoctorMetrics fetches the retrieval-health report from the local daemon.
func (c *Client) DoctorMetrics(ctx context.Context) (DoctorMetrics, error) {
	var m DoctorMetrics
	endpoint := c.baseURL + "/api/v1/doctor/metrics"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return m, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return m, fmt.Errorf("doctor metrics request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return m, apiError("doctor metrics", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return m, fmt.Errorf("decode doctor metrics: %w", err)
	}
	return m, nil
}

// DoctorArchiveAction performs the explicit, user-approved archive/restore
// mutation. action is "archive" or "restore"; empty uris on archive means
// bulk-archive the proposed set; all=true on restore un-archives everything.
// Returns the number of rows affected.
func (c *Client) DoctorArchiveAction(ctx context.Context, action string, uris []string, all bool) (int, error) {
	payload, _ := json.Marshal(map[string]any{"action": action, "uris": uris, "all": all})
	endpoint := c.baseURL + "/api/v1/doctor/archive"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s request: %w", action, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return 0, apiError("doctor archive", resp.StatusCode, body)
	}
	var out map[string]int
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("decode archive response: %w", err)
	}
	if action == "restore" {
		return out["restored"], nil
	}
	return out["archived"], nil
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

// BackfillProvenance invokes the daemon's one-time provenance backfill
// (issue #31). q holds optional threshold/max-scenes/dry-run/categories.
func (c *Client) BackfillProvenance(ctx context.Context, q url.Values) (string, error) {
	endpoint := c.baseURL + "/api/v1/maintenance/backfill-provenance?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("backfill provenance: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", apiError("backfill-provenance", resp.StatusCode, body)
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
