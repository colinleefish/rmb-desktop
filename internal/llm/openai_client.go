package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	log        *slog.Logger
	// Prompt template generations (0 = latest). Pinned versions let the
	// debug dry-run endpoints A/B replay sessions distilled with older
	// prompts (issue #28).
	extractVersion int
	distillVersion int
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

func NewOpenAICompatibleClient(cfg config.LLMConfig, log *slog.Logger) (*OpenAICompatibleClient, error) {
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
	if log == nil {
		log = slog.Default()
	}
	timeout := cfg.RequestTimeout()
	return &OpenAICompatibleClient{
		baseURL:    strings.TrimRight(base, "/"),
		apiKey:     key,
		model:      model,
		maxRetries: defaultMaxRetries,
		httpClient: &http.Client{Timeout: timeout},
		log:        log,
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
	system, user, err := ExtractPromptPair(c.extractVersion, messagesJSONL)
	if err != nil {
		return "", fmt.Errorf("llm extract atoms: %w", err)
	}
	req := chatCompletionRequest{
		Model:       c.model,
		Temperature: 0.1,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}
	out, err := c.completeWithRetry(ctx, "extract_atoms", req)
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
	out, err := c.completeWithRetry(ctx, "build_scenes", req)
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
	out, err := c.completeWithRetry(ctx, "session_abstract", req)
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
	related []RelatedEvent,
) (string, error) {
	version := c.distillVersion
	if version == 0 {
		version = DistillMemoryPromptLatest
	}
	req := chatCompletionRequest{
		Model:       c.model,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: distillMemorySystemPrompt},
			{Role: "user", Content: buildDistillMemoryPromptV(version, category, slug, atomsJSON, corrections, related)},
		},
	}
	out, err := c.completeWithRetry(ctx, "distill_memory", req)
	if err != nil {
		return "", fmt.Errorf("llm distill memory failed: %w", err)
	}
	return out, nil
}

// SetExtractPromptVersion pins the extraction prompt template generation used
// by this client (0 = latest). Older generations stay available for A/B
// session replay (issue #28).
func (c *OpenAICompatibleClient) SetExtractPromptVersion(version int) error {
	if _, _, err := ExtractPromptPair(version, ""); err != nil {
		return err
	}
	c.extractVersion = version
	return nil
}

// SetDistillPromptVersion pins the distillation prompt template generation
// used by this client (0 = latest). Older generations stay available for A/B
// session replay (issue #28).
func (c *OpenAICompatibleClient) SetDistillPromptVersion(version int) error {
	if version == 0 {
		c.distillVersion = 0
		return nil
	}
	if version < 1 || version > DistillMemoryPromptLatest {
		return fmt.Errorf("unknown distill prompt version %d (latest %d)", version, DistillMemoryPromptLatest)
	}
	c.distillVersion = version
	return nil
}

// DistillPromptVersion reports the effective distill prompt generation (0
// means latest, resolved at call time).
func (c *OpenAICompatibleClient) DistillPromptVersion() int {
	if c.distillVersion == 0 {
		return DistillMemoryPromptLatest
	}
	return c.distillVersion
}

func (c *OpenAICompatibleClient) completeWithRetry(ctx context.Context, operation string, req chatCompletionRequest) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		start := time.Now()
		content, status, reqBytes, respBytes, retryable, err := c.chatCompletion(ctx, req)
		c.logLLMRequest(operation, attempt, status, reqBytes, respBytes, time.Since(start), err)
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

func (c *OpenAICompatibleClient) logLLMRequest(operation string, attempt, status, reqBytes, respBytes int, latency time.Duration, err error) {
	if c.log == nil {
		return
	}
	attrs := []any{
		"op", operation,
		"attempt", attempt,
		"model", c.model,
		"host", c.traceHost(),
		"latency_ms", latency.Milliseconds(),
		"req_bytes", reqBytes,
	}
	if status > 0 {
		attrs = append(attrs, "status", status)
	}
	if respBytes > 0 {
		attrs = append(attrs, "resp_bytes", respBytes)
	}
	if err != nil {
		attrs = append(attrs, "err", err)
		c.log.Warn("llm request failed", attrs...)
		return
	}
	c.log.Info("llm request", attrs...)
}

func (c *OpenAICompatibleClient) traceHost() string {
	u, err := url.Parse(c.baseURL)
	if err != nil || u.Host == "" {
		return c.baseURL
	}
	return u.Host
}

func (c *OpenAICompatibleClient) chatCompletion(ctx context.Context, reqBody chatCompletionRequest) (content string, status, reqBytes, respBytes int, retryable bool, err error) {
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, 0, 0, false, fmt.Errorf("marshal llm request: %w", err)
	}
	reqBytes = len(raw)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", 0, reqBytes, 0, false, fmt.Errorf("build llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, reqBytes, 0, true, fmt.Errorf("perform llm request: %w", err)
	}
	defer resp.Body.Close()
	status = resp.StatusCode

	body, readErr := io.ReadAll(resp.Body)
	respBytes = len(body)
	if readErr != nil {
		return "", status, reqBytes, respBytes, false, fmt.Errorf("read llm response: %w", readErr)
	}

	if status >= 400 {
		retryable = status == http.StatusTooManyRequests || status >= 500
		errSnippet := body
		if len(errSnippet) > maxErrorBodyBytes {
			errSnippet = errSnippet[:maxErrorBodyBytes]
		}
		return "", status, reqBytes, respBytes, retryable, fmt.Errorf("llm http %d: %s", status, strings.TrimSpace(string(errSnippet)))
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return "", status, reqBytes, respBytes, false, fmt.Errorf("decode llm response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return "", status, reqBytes, respBytes, false, errors.New("llm response has no choices")
	}
	content = strings.TrimSpace(completion.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(completion.Choices[0].Message.ReasoningContent)
	}
	if content == "" {
		return "", status, reqBytes, respBytes, false, errors.New("llm response is empty")
	}
	return content, status, reqBytes, respBytes, false, nil
}
