package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
)

const testConnectionTimeout = 20 * time.Second
const maxModelsInTestResponse = 40

// LLMConnectionTestResult is the outcome of listing models via GET /models.
type LLMConnectionTestResult struct {
	Latency        time.Duration
	RequestedModel string
	Models         []string
	ModelsTotal    int
	ModelFound     bool
}

func (r LLMConnectionTestResult) Err() error {
	if r.RequestedModel == "" || r.ModelFound {
		return nil
	}
	return fmt.Errorf(
		"model %q not found in provider model list (%d models returned)",
		r.RequestedModel,
		r.ModelsTotal,
	)
}

// TestLLMConnection lists models via OpenAI-compatible GET /models (requires API key).
// When a model name is configured, ModelFound reflects whether it appears in the list.
func TestLLMConnection(ctx context.Context, cfg config.LLMConfig) (LLMConnectionTestResult, error) {
	start := time.Now()
	ids, err := listModels(ctx, cfg.APIBase, cfg.APIKey, testConnectionTimeout)
	if err != nil {
		return LLMConnectionTestResult{}, err
	}

	model := strings.TrimSpace(cfg.Model)
	out := LLMConnectionTestResult{
		Latency:        time.Since(start),
		RequestedModel: model,
		ModelsTotal:    len(ids),
		Models:         capModelIDs(ids, maxModelsInTestResponse),
		ModelFound:     modelsContain(ids, model),
	}
	if err := out.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func capModelIDs(ids []string, max int) []string {
	if len(ids) <= max {
		return ids
	}
	return ids[:max]
}

// TestEmbedConnection performs a minimal embedding request against the provider.
func TestEmbedConnection(ctx context.Context, cfg config.EmbedConfig) (time.Duration, error) {
	client, err := NewEmbeddingClient(cfg)
	if err != nil {
		return 0, err
	}
	client.maxRetries = 1
	client.httpClient.Timeout = testConnectionTimeout

	start := time.Now()
	_, err = client.Embed(ctx, []string{"test"})
	return time.Since(start), err
}
