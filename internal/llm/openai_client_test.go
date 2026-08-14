package llm

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/debug"
)

func TestOpenAICompatibleClientLogsRequestTrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{{Message: chatMessage{Role: "assistant", Content: `{"scenes":[]}`}}},
		})
	}))
	defer srv.Close()

	buf := debug.NewLogBuffer(20)
	log := debug.NewLogger(buf, slog.NewJSONHandler(discardWriter{}, nil))
	client, err := NewOpenAICompatibleClient(config.LLMConfig{
		APIBase: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}, log)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.BuildScenes(context.Background(), `{"atoms":[]}`)
	if err != nil {
		t.Fatalf("BuildScenes: %v", err)
	}

	entries := buf.Tail(10, "", "")
	found := false
	for _, e := range entries {
		if e.Message != "llm request" {
			continue
		}
		if e.Attrs["op"] != "build_scenes" {
			t.Fatalf("op = %#v", e.Attrs["op"])
		}
		if e.Attrs["status"] == nil {
			t.Fatalf("missing status in %#v", e.Attrs)
		}
		found = true
	}
	if !found {
		t.Fatalf("missing llm request log: %#v", entries)
	}
}

func TestOpenAICompatibleClientLogsFailedRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	buf := debug.NewLogBuffer(20)
	log := debug.NewLogger(buf, slog.NewJSONHandler(discardWriter{}, nil))
	client, err := NewOpenAICompatibleClient(config.LLMConfig{
		APIBase: srv.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}, log)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.BuildScenes(context.Background(), `{"atoms":[]}`)
	if err == nil {
		t.Fatal("expected error")
	}

	entries := buf.Tail(10, "", "")
	var failed []string
	for _, e := range entries {
		if e.Message == "llm request failed" {
			errText, _ := e.Attrs["err"].(string)
			if !strings.Contains(errText, "429") {
				t.Fatalf("err = %q", errText)
			}
			failed = append(failed, errText)
		}
	}
	if len(failed) < 2 {
		t.Fatalf("expected retry logs, got %#v", entries)
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
