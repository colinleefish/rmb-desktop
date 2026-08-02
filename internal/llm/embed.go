package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
)

type EmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimensions int
	maxRetries int
	httpClient *http.Client
}

func NewEmbeddingClient(cfg config.EmbedConfig) (*EmbeddingClient, error) {
	base := strings.TrimSpace(cfg.APIBase)
	key := strings.TrimSpace(cfg.APIKey)
	model := strings.TrimSpace(cfg.Model)
	if base == "" {
		return nil, errors.New("embed api_base is required")
	}
	if key == "" {
		return nil, errors.New("embed api_key is required")
	}
	if model == "" {
		return nil, errors.New("embed model is required")
	}
	dims := cfg.Dimensions
	if dims <= 0 {
		dims = 1024
	}
	return &EmbeddingClient{
		baseURL:    strings.TrimRight(base, "/"),
		apiKey:     key,
		model:      model,
		dimensions: dims,
		maxRetries: 3,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *EmbeddingClient) Dimensions() int { return c.dimensions }

type embeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (c *EmbeddingClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	reqBody := embeddingRequest{
		Model:      c.model,
		Input:      inputs,
		Dimensions: c.dimensions,
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		vectors, retryable, err := c.embedOnce(ctx, reqBody, len(inputs))
		if err == nil {
			return vectors, nil
		}
		lastErr = err
		if !retryable || attempt == c.maxRetries {
			break
		}
		backoff := time.Duration(attempt*attempt) * 300 * time.Millisecond
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func (c *EmbeddingClient) embedOnce(ctx context.Context, reqBody embeddingRequest, want int) ([][]float32, bool, error) {
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, false, fmt.Errorf("marshal embed request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, false, fmt.Errorf("build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("perform embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retryable, fmt.Errorf("embed http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false, fmt.Errorf("decode embed response: %w", err)
	}
	if len(out.Data) != want {
		return nil, false, fmt.Errorf("embed response count %d != inputs %d", len(out.Data), want)
	}

	vectors := make([][]float32, want)
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= want {
			return nil, false, fmt.Errorf("embed response index %d out of range", d.Index)
		}
		if len(d.Embedding) != c.dimensions {
			return nil, false, fmt.Errorf("embed dim %d != expected %d", len(d.Embedding), c.dimensions)
		}
		vectors[d.Index] = d.Embedding
	}
	for i, v := range vectors {
		if v == nil {
			return nil, false, fmt.Errorf("embed response missing index %d", i)
		}
	}
	return vectors, false, nil
}
