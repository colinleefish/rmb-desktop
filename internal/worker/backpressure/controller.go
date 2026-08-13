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

// Controller is a backlog-driven concurrency limiter for per-stage session
// workers. Concurrency follows the pending queue size directly (clamped to
// [min, max]) and cuts hard on LLM pressure.
type Controller struct {
	mu      sync.Mutex
	min     int
	max     int
	current int
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

// SeedFromBacklog sets initial concurrency from the pending queue size.
func (c *Controller) SeedFromBacklog(pending int) (prev, next int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev = c.current
	c.current = c.clamp(pending)
	return prev, c.current
}

// Observe records one job result for the current cycle window.
func (c *Controller) Observe(o Outcome) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.window = append(c.window, o)
}

// EndCycle sets concurrency from the remaining pending hint, halving on pressure.
func (c *Controller) EndCycle(pendingHint int) (prev, next int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prev = c.current
	pressure := false
	for _, o := range c.window {
		if o.hasPressure() {
			pressure = true
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

	// Follow the backlog directly: concurrency tracks pending work.
	c.current = c.clamp(pendingHint)
	return prev, c.current
}

func (c *Controller) clamp(pending int) int {
	if pending > c.max {
		pending = c.max
	}
	if pending < c.min {
		pending = c.min
	}
	return pending
}
