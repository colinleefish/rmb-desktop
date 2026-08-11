package llm

import (
	"context"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
)

const testConnectionTimeout = 20 * time.Second
const maxModelsInTestResponse = 40

// LLMConnectionTestResult is the outcome of listing models via GET /models.
type LLMConnectionTestResult struct {
	Latency     time.Duration
	Models      []string
	ModelsTotal int
}

// TestLLMConnection verifies connectivity by listing models (OpenAI-compatible GET /models).
func TestLLMConnection(ctx context.Context, cfg config.LLMConfig) (LLMConnectionTestResult, error) {
	start := time.Now()
	ids, err := listModels(ctx, cfg.APIBase, cfg.APIKey, testConnectionTimeout)
	if err != nil {
		return LLMConnectionTestResult{}, err
	}

	return LLMConnectionTestResult{
		Latency:     time.Since(start),
		ModelsTotal: len(ids),
		Models:      capModelIDs(ids, maxModelsInTestResponse),
	}, nil
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
