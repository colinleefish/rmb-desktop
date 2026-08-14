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

const (
	defaultMaxRetries = 2
	maxErrorBodyBytes = 2048
)

type OpenAICompatibleConfig struct {
	APIBase    string
	APIKey     string
	Model      string
	MaxRetries int
	Timeout    time.Duration
}

type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	maxRetries int
	httpClient *http.Client
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Messages    []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func NewOpenAICompatibleClient(cfg config.LLMConfig) (*OpenAICompatibleClient, error) {
	base := strings.TrimSpace(cfg.APIBase)
	key := strings.TrimSpace(cfg.APIKey)
	model := strings.TrimSpace(cfg.Model)
	if base == "" {
		return nil, errors.New("llm api_base is required")
	}
	if key == "" {
		return nil, errors.New("llm api_key is required")
	}
	if model == "" {
		return nil, errors.New("llm model is required")
	}
	timeout := cfg.RequestTimeout()
	return &OpenAICompatibleClient{
		baseURL:    strings.TrimRight(base, "/"),
		apiKey:     key,
		model:      model,
		maxRetries: defaultMaxRetries,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// DebugRequestBudget estimates how long a debug endpoint should wait for N LLM calls.
func DebugRequestBudget(cfg config.LLMConfig, llmCalls int) time.Duration {
	if llmCalls < 1 {
		llmCalls = 1
	}
	perCall := cfg.RequestTimeout() * time.Duration(defaultMaxRetries)
	return perCall*time.Duration(llmCalls) + 15*time.Second
}

func (c *OpenAICompatibleClient) ExtractAtoms(ctx context.Context, messagesJSONL string) (string, error) {
	req := chatCompletionRequest{
		Model:       c.model,
		Temperature: 0.1,
		Messages: []chatMessage{
			{Role: "system", Content: extractAtomsSystemPrompt},
			{Role: "user", Content: buildExtractAtomsPrompt(messagesJSONL)},
		},
	}
	out, err := c.completeWithRetry(ctx, req)
	if err != nil {
		return "", fmt.Errorf("llm extract atoms failed: %w", err)
	}
	return out, nil
}

func (c *OpenAICompatibleClient) BuildScenes(ctx context.Context, atomsJSON string) (string, error) {
	req := chatCompletionRequest{
		Model:       c.model,
		Temperature: 0.1,
		Messages: []chatMessage{
			{Role: "system", Content: buildScenesSystemPrompt},
			{Role: "user", Content: buildBuildScenesPrompt(atomsJSON)},
		},
	}
	out, err := c.completeWithRetry(ctx, req)
	if err != nil {
		return "", fmt.Errorf("llm build scenes failed: %w", err)
	}
	return out, nil
}

func (c *OpenAICompatibleClient) SummarizeSessionAbstract(ctx context.Context, sceneAbstracts string) (string, error) {
	req := chatCompletionRequest{
		Model:       c.model,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: sessionAbstractSystemPrompt},
			{Role: "user", Content: buildSessionAbstractPrompt(sceneAbstracts)},
		},
	}
	out, err := c.completeWithRetry(ctx, req)
	if err != nil {
		return "", fmt.Errorf("llm summarize session abstract failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (c *OpenAICompatibleClient) DistillMemory(
	ctx context.Context,
	category string,
	slug string,
	atomsJSON string,
	corrections []string,
) (string, error) {
	req := chatCompletionRequest{
		Model:       c.model,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: distillMemorySystemPrompt},
			{Role: "user", Content: buildDistillMemoryPrompt(category, slug, atomsJSON, corrections)},
		},
	}
	out, err := c.completeWithRetry(ctx, req)
	if err != nil {
		return "", fmt.Errorf("llm distill memory failed: %w", err)
	}
	return out, nil
}

func (c *OpenAICompatibleClient) completeWithRetry(ctx context.Context, req chatCompletionRequest) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		content, retryable, err := c.chatCompletion(ctx, req)
		if err == nil {
			return content, nil
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
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", lastErr
}

func (c *OpenAICompatibleClient) chatCompletion(ctx context.Context, reqBody chatCompletionRequest) (string, bool, error) {
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", false, fmt.Errorf("marshal llm request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", false, fmt.Errorf("build llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("perform llm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", retryable, fmt.Errorf("llm http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var completion chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return "", false, fmt.Errorf("decode llm response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", false, errors.New("llm response has no choices")
	}
	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(completion.Choices[0].Message.ReasoningContent)
	}
	if content == "" {
		return "", false, errors.New("llm response is empty")
	}
	return content, false, nil
}
