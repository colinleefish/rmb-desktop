package shared

import (
	"context"
	"log/slog"

	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/worker/backpressure"
)

// CycleDeps supplies the stage-specific queries and per-session work for
// RunBackpressuredCycle.
type CycleDeps struct {
	// SelectCandidates returns up to ~2x max-concurrency session IDs to try
	// this cycle, oldest work first.
	SelectCandidates func(ctx context.Context) ([]string, error)
	// CountPending returns the stage's backlog size, used to seed and steer
	// concurrency.
	CountPending func(ctx context.Context) (int, error)
	// ProcessSession runs one session end to end (prepare, LLM, persist).
	ProcessSession func(ctx context.Context, sessionID string) error
}

// RunBackpressuredCycle is the shared L1/L2 scheduling loop: it selects
// candidate sessions, seeds concurrency from the backlog, then processes the
// candidates in limit-sized parallel batches. After each batch the controller
// observes every outcome and recomputes concurrency from the pending hint;
// the cycle stops early when concurrency drops (LLM pressure) so the worker
// does not keep hammering the provider.
func RunBackpressuredCycle(ctx context.Context, name string, bp *backpressure.Controller, reg *debug.Registry, log *slog.Logger, d CycleDeps) {
	endCycle := reg.BeginCycle(name)
	defer endCycle(nil)

	ids, err := d.SelectCandidates(ctx)
	if err != nil {
		log.Error(name+" select candidates failed", "err", err)
		return
	}
	if len(ids) == 0 {
		bp.EndCycle(0)
		return
	}

	if n, err := d.CountPending(ctx); err == nil {
		if prev, next := bp.SeedFromBacklog(n); prev != next {
			log.Info(name+" concurrency seeded", "from", prev, "to", next, "pending", n)
		}
	}

	limit := bp.Limit()
	reg.SetConcurrency(name, limit)
	log.Info(name+" cycle", "candidates", len(ids), "concurrency", limit)

	remaining := ids
	for len(remaining) > 0 {
		if ctx.Err() != nil {
			return
		}
		limit = bp.Limit()
		n := limit
		if n > len(remaining) {
			n = len(remaining)
		}
		batch := remaining[:n]
		remaining = remaining[n:]

		backpressure.RunParallel(ctx, batch, limit, func(ctx context.Context, id string) {
			err := d.ProcessSession(ctx, id)
			bp.Observe(backpressure.Outcome{Err: err})
			if err != nil && !llm.IsTransientError(err) {
				log.Error(name+" process session failed", "session_id", id, "err", err)
			}
		})

		pendingHint := len(remaining)
		if n, err := d.CountPending(ctx); err == nil {
			pendingHint = n
		}
		prev, next := bp.EndCycle(pendingHint)
		reg.SetBackpressure(name, bp.Min(), bp.Max(), next, pendingHint)
		if prev != next {
			log.Info(name+" concurrency adjusted", "from", prev, "to", next, "pending", pendingHint)
		}
		// Stop this cycle on pressure so we don't keep hammering the LLM.
		if next < prev {
			return
		}
	}
}
