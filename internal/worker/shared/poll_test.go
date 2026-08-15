package shared

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/debug"
)

func TestRunPollLifecycle(t *testing.T) {
	reg := debug.NewRegistry()
	buf := debug.NewLogBuffer(20)
	log := debug.NewLogger(buf, slog.NewTextHandler(io.Discard, nil))

	var cycles int32
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunPoll(ctx, PollOptions{
			Name:       "l9",
			Label:      "l9 test",
			Interval:   5 * time.Millisecond,
			Registry:   reg,
			Log:        log,
			StartAttrs: []any{"extra", 1},
			Cycle:      func(context.Context) { atomic.AddInt32(&cycles, 1) },
		})
	}()

	// Wait for several ticker-driven cycles, then cancel.
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&cycles) < 3 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	w, ok := reg.Snapshot().Workers["l9"]
	if !ok || !w.Alive {
		t.Fatalf("expected worker l9 alive during poll, got %+v", w)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunPoll did not return after cancel")
	}

	if w, ok := reg.Snapshot().Workers["l9"]; !ok || w.Alive {
		t.Fatalf("expected worker l9 stopped after cancel, got %+v", w)
	}

	entries := buf.Tail(20, "", "")
	var started, stopped bool
	for _, e := range entries {
		if e.Message == "l9 test worker started" {
			started = true
		}
		if e.Message == "l9 test worker stopped" {
			stopped = true
		}
	}
	if !started || !stopped {
		t.Fatalf("missing lifecycle logs (started=%v stopped=%v): %#v", started, stopped, entries)
	}
	if got := atomic.LoadInt32(&cycles); got < 2 {
		t.Fatalf("expected immediate first cycle plus ticker cycles, got %d", got)
	}
}
