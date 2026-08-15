package shared

import (
	"context"
	"log/slog"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/debug"
)

// PollOptions configures RunPoll.
type PollOptions struct {
	// Name is the worker's registry name (e.g. "l1", "embed").
	Name string
	// Label prefixes the start/stop log messages (e.g. "l1 extract").
	Label string
	// Interval is the poll cadence between cycles.
	Interval time.Duration
	// Registry tracks worker liveness.
	Registry *debug.Registry
	// Log receives the start/stop lifecycle messages.
	Log *slog.Logger
	// StartAttrs are extra attributes appended after poll_interval on the
	// "worker started" log line (e.g. concurrency bounds).
	StartAttrs []any
	// Cycle is one poll iteration; it runs once immediately and then on every
	// tick of Interval until ctx is cancelled.
	Cycle func(context.Context)
}

// RunPoll drives a stage worker: it registers the worker, logs "<Label> worker
// started", runs Cycle immediately and then on every tick of Interval, and
// stops with a final log line when ctx is cancelled.
func RunPoll(ctx context.Context, o PollOptions) {
	o.Registry.WorkerStarted(o.Name)
	defer o.Registry.WorkerStopped(o.Name)

	attrs := append([]any{"poll_interval", o.Interval}, o.StartAttrs...)
	o.Log.Info(o.Label+" worker started", attrs...)

	o.Cycle(ctx)

	ticker := time.NewTicker(o.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			o.Log.Info(o.Label + " worker stopped")
			return
		case <-ticker.C:
			o.Cycle(ctx)
		}
	}
}
