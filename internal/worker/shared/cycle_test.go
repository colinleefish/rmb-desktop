package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/worker/backpressure"
)

func testCycleLogger() (*debug.LogBuffer, *slog.Logger) {
	buf := debug.NewLogBuffer(50)
	return buf, debug.NewLogger(buf, slog.NewTextHandler(io.Discard, nil))
}

func TestRunBackpressuredCycleProcessesAllBatches(t *testing.T) {
	reg := debug.NewRegistry()
	_, log := testCycleLogger()
	bp := backpressure.New(1, 2)

	ids := []string{"a", "b", "c", "d", "e"}
	var mu sync.Mutex
	processed := map[string]bool{}

	RunBackpressuredCycle(context.Background(), "t1", bp, reg, log, CycleDeps{
		SelectCandidates: func(context.Context) ([]string, error) { return ids, nil },
		CountPending:     func(context.Context) (int, error) { return len(ids), nil },
		ProcessSession: func(_ context.Context, id string) error {
			mu.Lock()
			defer mu.Unlock()
			processed[id] = true
			return nil
		},
	})

	if len(processed) != len(ids) {
		t.Fatalf("processed %d of %d candidates: %v", len(processed), len(ids), processed)
	}
	snap, ok := reg.BackpressureSnapshot().Workers["t1"]
	if !ok {
		t.Fatal("missing backpressure snapshot for t1")
	}
	if snap.Min != 1 || snap.Max != 2 || snap.Current != 2 {
		t.Fatalf("unexpected controller bounds/current: %+v", snap)
	}
}

func TestRunBackpressuredCycleStopsOnPressure(t *testing.T) {
	reg := debug.NewRegistry()
	_, log := testCycleLogger()
	bp := backpressure.New(1, 2)

	ids := []string{"a", "b", "c", "d"}
	var mu sync.Mutex
	var calls int
	deps := CycleDeps{
		CountPending: func(context.Context) (int, error) { return len(ids), nil },
		ProcessSession: func(_ context.Context, _ string) error {
			mu.Lock()
			calls++
			mu.Unlock()
			return errors.New("HTTP 429 rate limit")
		},
	}
	deps.SelectCandidates = func(context.Context) ([]string, error) { return ids, nil }

	RunBackpressuredCycle(context.Background(), "t2", bp, reg, log, deps)

	// Seeded at max=2, the first batch of 2 fails transiently; EndCycle halves
	// concurrency (2 -> 1) and the cycle must stop instead of continuing.
	mu.Lock()
	defer mu.Unlock()
	if calls > 2 {
		t.Fatalf("cycle kept running under pressure: %d sessions processed", calls)
	}
	if calls == 0 {
		t.Fatal("expected the first batch to run")
	}
}

func TestRunBackpressuredCycleEmptyCandidates(t *testing.T) {
	reg := debug.NewRegistry()
	_, log := testCycleLogger()
	bp := backpressure.New(1, 2)

	called := false
	RunBackpressuredCycle(context.Background(), "t3", bp, reg, log, CycleDeps{
		SelectCandidates: func(context.Context) ([]string, error) { return nil, nil },
		CountPending:     func(context.Context) (int, error) { return 0, nil },
		ProcessSession: func(context.Context, string) error {
			called = true
			return nil
		},
	})
	if called {
		t.Fatal("processSession must not run with no candidates")
	}
	if got := bp.Limit(); got != 1 {
		t.Fatalf("EndCycle(0) should clamp to min, got concurrency %d", got)
	}
}

func TestRunBackpressuredCycleSelectErrorLogsAndStops(t *testing.T) {
	reg := debug.NewRegistry()
	buf, log := testCycleLogger()
	bp := backpressure.New(1, 2)

	called := false
	RunBackpressuredCycle(context.Background(), "t4", bp, reg, log, CycleDeps{
		SelectCandidates: func(context.Context) ([]string, error) { return nil, fmt.Errorf("boom") },
		CountPending:     func(context.Context) (int, error) { return 0, nil },
		ProcessSession: func(context.Context, string) error {
			called = true
			return nil
		},
	})
	if called {
		t.Fatal("processSession must not run after select failure")
	}
	found := false
	for _, e := range buf.Tail(20, "", "") {
		if e.Message == "t4 select candidates failed" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing select-failure log")
	}
}
