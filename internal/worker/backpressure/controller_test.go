package backpressure_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/worker/backpressure"
)

func TestControllerAIMD(t *testing.T) {
	c := backpressure.New(1, 4)
	if c.Limit() != 1 {
		t.Fatalf("start=%d want 1", c.Limit())
	}

	// Healthy backlog → scale up one per cycle.
	for want := 2; want <= 4; want++ {
		c.Observe(backpressure.Outcome{})
		c.Observe(backpressure.Outcome{})
		_, next := c.EndCycle(20)
		if next != want {
			t.Fatalf("scale up next=%d want %d", next, want)
		}
	}

	// Cap at max.
	c.Observe(backpressure.Outcome{})
	_, next := c.EndCycle(20)
	if next != 4 {
		t.Fatalf("capped next=%d want 4", next)
	}

	// Pressure → halve (4 → 2).
	c.Observe(backpressure.Outcome{Pressure: true})
	_, next = c.EndCycle(20)
	if next != 2 {
		t.Fatalf("after pressure next=%d want 2", next)
	}

	// 429-shaped error also counts as pressure (2 → 1).
	c.Observe(backpressure.Outcome{Err: errors.New("llm http 429: rate limit")})
	_, next = c.EndCycle(20)
	if next != 1 {
		t.Fatalf("after 429 next=%d want 1", next)
	}

	// Quiet queue cools toward min (already min).
	c.Observe(backpressure.Outcome{})
	_, next = c.EndCycle(1)
	if next != 1 {
		t.Fatalf("quiet next=%d want 1", next)
	}
}

func TestControllerCoolDown(t *testing.T) {
	c := backpressure.New(1, 8)
	for i := 0; i < 3; i++ {
		c.Observe(backpressure.Outcome{})
		c.EndCycle(100)
	}
	if c.Limit() != 4 {
		t.Fatalf("warm limit=%d want 4", c.Limit())
	}
	c.Observe(backpressure.Outcome{})
	_, next := c.EndCycle(1) // pendingHint <= min → cool down
	if next != 3 {
		t.Fatalf("cooldown next=%d want 3", next)
	}
}

func TestControllerSeedFromBacklog(t *testing.T) {
	c := backpressure.New(1, 8)
	_, next := c.SeedFromBacklog(500)
	if next < 4 {
		t.Fatalf("large backlog seed=%d want >= 4", next)
	}
	if next > 8 {
		t.Fatalf("seed=%d exceeds max", next)
	}
	_, next2 := c.SeedFromBacklog(500)
	if next2 != next {
		t.Fatalf("second seed should no-op, got %d want %d", next2, next)
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
