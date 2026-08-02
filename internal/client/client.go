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

func (c *Client) Search(ctx context.Context, query string, k int, scopes []string) ([]Match, error) {
	q := url.Values{}
	q.Set("q", query)
	if k > 0 {
		q.Set("k", strconv.Itoa(k))
	}
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, ","))
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
	q := url.Values{}
	q.Set("uri", uri)
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

func apiError(path string, status int, body []byte) error {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return fmt.Errorf("%s: %s", path, e.Error)
	}
	return fmt.Errorf("%s returned %d", path, status)
}
