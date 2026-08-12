package backpressure

import (
	"sync"

	"github.com/colinleefish/rmb-desktop/internal/llm"
)

// Outcome is one session job result used to steer concurrency.
type Outcome struct {
	Err      error
	Pressure bool // explicit pressure (e.g. 429); also inferred from Err via llm.IsTransientError
}

func (o Outcome) hasPressure() bool {
	return o.Pressure || llm.IsTransientError(o.Err)
}

// Controller is an AIMD concurrency limiter for per-stage session workers.
// Scale up when backlog is healthy; cut hard on LLM pressure.
type Controller struct {
	mu      sync.Mutex
	min     int
	max     int
	current int
	seeded  bool
	window  []Outcome
}

func New(min, max int) *Controller {
	if min < 1 {
		min = 1
	}
	if max < min {
		max = min
	}
	return &Controller{min: min, max: max, current: min}
}

func (c *Controller) Limit() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

func (c *Controller) Min() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.min
}

func (c *Controller) Max() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.max
}

// SeedFromBacklog raises initial concurrency when a large queue is waiting.
// Runs at most once so AIMD back pressure remains in control afterward.
func (c *Controller) SeedFromBacklog(pending int) (prev, next int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev = c.current
	if c.seeded {
		return prev, prev
	}
	c.seeded = true
	if pending <= c.current {
		return prev, prev
	}
	// Aim for enough parallelism to chew backlog, but stay conservative.
	target := c.min + pending/64
	if target < c.min+1 && pending > c.max {
		target = c.min + 1
	}
	if pending >= c.max*4 {
		// Large import / backfill: jump toward half-max at least.
		half := c.max / 2
		if half < c.min {
			half = c.min
		}
		if target < half {
			target = half
		}
	}
	if target > c.max {
		target = c.max
	}
	if target < c.min {
		target = c.min
	}
	c.current = target
	return prev, c.current
}

// Observe records one job result for the current cycle window.
func (c *Controller) Observe(o Outcome) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.window = append(c.window, o)
}

// EndCycle applies AIMD using the window and remaining/known pending hint.
// pendingHint should be how much work is still queued (including just-finished batch size is fine).
func (c *Controller) EndCycle(pendingHint int) (prev, next int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prev = c.current
	pressure := false
	ok := 0
	for _, o := range c.window {
		if o.hasPressure() {
			pressure = true
		} else if o.Err == nil {
			ok++
		}
	}
	c.window = nil

	if pressure {
		// Multiplicative decrease.
		next = c.current / 2
		if next < c.min {
			next = c.min
		}
		c.current = next
		return prev, next
	}

	next = c.current
	// Additive increase while backlog exceeds current concurrency.
	if pendingHint > c.current && ok > 0 && c.current < c.max {
		next = c.current + 1
	}
	// Cool down toward min when the queue is drained.
	if pendingHint <= c.min && c.current > c.min {
		next = c.current - 1
		if next < c.min {
			next = c.min
		}
	}
	c.current = next
	return prev, next
}
