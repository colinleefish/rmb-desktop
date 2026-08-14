package debug

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestAttrToJSONErrorString(t *testing.T) {
	err := fmt.Errorf("llm build scenes failed: %w", errors.New("timeout"))
	got := attrToJSON(slog.Any("err", err))
	s, ok := got.(string)
	if !ok {
		t.Fatalf("expected string, got %T %#v", got, got)
	}
	if !strings.Contains(s, "llm build scenes failed") || !strings.Contains(s, "timeout") {
		t.Fatalf("unexpected error text: %q", s)
	}
}

func TestLogBufferCapturesErrorText(t *testing.T) {
	buf := NewLogBuffer(10)
	logger := NewLogger(buf, slog.NewJSONHandler(discardWriter{}, nil))
	logger.Error("l2 process session failed", "session_id", "abc", "err", fmt.Errorf("parse build scenes: no usable scenes"))

	entries := buf.Tail(1, "", "")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	errVal, ok := entries[0].Attrs["err"]
	if !ok {
		t.Fatal("missing err attr")
	}
	if errVal != "parse build scenes: no usable scenes" {
		t.Fatalf("err attr = %#v", errVal)
	}

	raw, err := json.Marshal(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"err":{}`) {
		t.Fatalf("json still contains empty error object: %s", raw)
	}
}

func TestAttrToJSONNilError(t *testing.T) {
	if got := anyToJSON(nil); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
	var err error
	if got := anyToJSON(err); got != nil {
		t.Fatalf("expected nil for typed nil error, got %#v", got)
	}
}

func TestBufferHandlerEnabledDelegates(t *testing.T) {
	buf := NewLogBuffer(1)
	h := buf.Handler(slog.NewJSONHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected info disabled when base level is error")
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
