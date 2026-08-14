package worker

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/pipeline"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
	"github.com/colinleefish/rmb-desktop/internal/worker/embed"
	"github.com/colinleefish/rmb-desktop/internal/worker/extract"
	"github.com/colinleefish/rmb-desktop/internal/worker/memory"
	"github.com/colinleefish/rmb-desktop/internal/worker/scene"
)

// Runner starts background distillation workers.
type Runner struct {
	cfg   config.Config
	db    *sql.DB
	log   *slog.Logger
	locks *workerlock.SessionLocks
	reg   *debug.Registry
	wg    sync.WaitGroup
}

func NewRunner(cfg config.Config, database *sql.DB, log *slog.Logger, reg *debug.Registry) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{
		cfg:   cfg,
		db:    database,
		log:   log,
		locks: workerlock.NewSessionLocks(),
		reg:   reg,
	}
}

// Start launches workers in background goroutines. Returns immediately.
func (r *Runner) Start(ctx context.Context) {
	if !r.cfg.DistillationEnabled() {
		r.log.Info("distillation disabled (no llm api key); ingest-only mode")
		return
	}

	if err := recoverPipelineState(ctx, r.db); err != nil {
		r.log.Warn("pipeline state recovery failed", "err", err)
	}
	if err := rebuildFTSIndexes(ctx, r.db); err != nil {
		r.log.Warn("fts index rebuild failed", "err", err)
	}

	chat, err := llm.NewOpenAICompatibleClient(r.cfg.LLM)
	if err != nil {
		r.log.Error("failed to create llm client", "err", err)
		return
	}

	r.startWorker(ctx, "l1", func(ctx context.Context) error {
		return extract.NewWorker(r.db, chat, r.cfg.Pipeline, r.locks, r.log, r.reg).Run(ctx)
	})
	r.startWorker(ctx, "l2", func(ctx context.Context) error {
		return scene.NewWorker(r.db, chat, r.cfg.Pipeline, r.locks, r.log, r.reg).Run(ctx)
	})
	r.startWorker(ctx, "l3", func(ctx context.Context) error {
		return memory.NewWorker(r.db, chat, r.cfg.Pipeline, r.log, r.reg).Run(ctx)
	})

	if r.cfg.Embed.HasKey() {
		embedClient, err := llm.NewEmbeddingClient(r.cfg.Embed)
		if err != nil {
			r.log.Error("failed to create embed client", "err", err)
		} else {
			r.startWorker(ctx, "embed", func(ctx context.Context) error {
				return embed.NewWorker(r.db, embedClient, r.cfg.Pipeline, r.cfg.Embed.Dimensions, r.log, r.reg).Run(ctx)
			})
		}
	} else {
		r.log.Info("embed worker disabled (no embed api key)")
	}
}

func (r *Runner) startWorker(ctx context.Context, name string, fn func(context.Context) error) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := fn(ctx); err != nil && ctx.Err() == nil {
			r.log.Error("worker exited", "name", name, "err", err)
		}
	}()
}

// Wait blocks until all workers stop (ctx cancelled).
func (r *Runner) Wait() {
	r.wg.Wait()
}

// recoverPipelineState resets worker states left mid-flight after a crash or restart.
func recoverPipelineState(ctx context.Context, db *sql.DB) error {
	for _, stage := range []pipeline.Stage{pipeline.StageL1, pipeline.StageL2, pipeline.StageL3} {
		if _, err := pipeline.ResetRunningToPending(ctx, db, stage); err != nil {
			return err
		}
	}
	return nil
}

func rebuildFTSIndexes(ctx context.Context, db *sql.DB) error {
	for _, table := range []string{"atoms_fts", "scenes_fts", "memories_fts"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO `+table+`(`+table+`) VALUES('rebuild')`); err != nil {
			return err
		}
	}
	return nil
}
