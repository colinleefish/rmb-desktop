package backpressure_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/worker/backpressure"
)

func TestControllerFollowsBacklog(t *testing.T) {
	c := backpressure.New(1, 4)
	if c.Limit() != 1 {
		t.Fatalf("start=%d want 1", c.Limit())
	}

	// Backlog exceeds max → jump straight to max (no additive ramp).
	c.Observe(backpressure.Outcome{})
	_, next := c.EndCycle(20)
	if next != 4 {
		t.Fatalf("follow backlog next=%d want 4", next)
	}

	// Backlog between min and max → track it exactly.
	c.Observe(backpressure.Outcome{})
	_, next = c.EndCycle(3)
	if next != 3 {
		t.Fatalf("follow backlog next=%d want 3", next)
	}

	// Pressure → halve (3 → 1, floored at min).
	c.Observe(backpressure.Outcome{Pressure: true})
	_, next = c.EndCycle(20)
	if next != 1 {
		t.Fatalf("after pressure next=%d want 1", next)
	}

	// No pressure → recover straight to backlog (20 → 4).
	c.Observe(backpressure.Outcome{})
	_, next = c.EndCycle(20)
	if next != 4 {
		t.Fatalf("recover next=%d want 4", next)
	}

	// Drained queue → clamp to min.
	c.Observe(backpressure.Outcome{})
	_, next = c.EndCycle(0)
	if next != 1 {
		t.Fatalf("drained next=%d want 1", next)
	}
}

func TestControllerSeedFromBacklog(t *testing.T) {
	c := backpressure.New(1, 8)
	_, next := c.SeedFromBacklog(500)
	if next != 8 {
		t.Fatalf("large backlog seed=%d want 8", next)
	}
	// Seed tracks backlog directly on each call (no one-shot behavior).
	_, next = c.SeedFromBacklog(3)
	if next != 3 {
		t.Fatalf("small backlog seed=%d want 3", next)
	}
}

func TestRunParallelRespectsLimit(t *testing.T) {
	var inFlight atomic.Int32
	var maxSeen atomic.Int32
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = string(rune('a' + i))
	}

	backpressure.RunParallel(context.Background(), ids, 3, func(_ context.Context, _ string) {
		cur := inFlight.Add(1)
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
	})

	if maxSeen.Load() > 3 {
		t.Fatalf("max in-flight=%d want <= 3", maxSeen.Load())
	}
	if maxSeen.Load() < 1 {
		t.Fatal("expected work to run")
	}
}
