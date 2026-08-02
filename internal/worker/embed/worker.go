package embed

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/llm"
)

type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

type Worker struct {
	db     *sql.DB
	llm    Embedder
	cfg    config.PipelineConfig
	dims   int
	log    *slog.Logger
}

func NewWorker(database *sql.DB, embedder Embedder, cfg config.PipelineConfig, dims int, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	if dims <= 0 {
		dims = 1024
	}
	return &Worker{db: database, llm: embedder, cfg: cfg, dims: dims, log: log}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.llm == nil {
		return fmt.Errorf("embed worker requires embedding client")
	}
	interval := w.cfg.EmbedPollInterval
	if interval <= 0 {
		return fmt.Errorf("invalid embed poll interval")
	}
	w.log.Info("embed worker started", "poll_interval", interval)
	w.runOneCycle(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("embed worker stopped")
			return nil
		case <-ticker.C:
			w.runOneCycle(ctx)
		}
	}
}

func (w *Worker) runOneCycle(ctx context.Context) {
	for _, tier := range []struct {
		name string
		fn   func(context.Context) (int, error)
	}{
		{"atoms", w.embedAtoms},
		{"scenes", w.embedScenes},
		{"memories", w.embedMemories},
	} {
		n, err := tier.fn(ctx)
		if err != nil {
			if llm.IsTransientError(err) {
				w.log.Warn("embed transient error", "tier", tier.name, "err", err)
			} else {
				w.log.Error("embed failed", "tier", tier.name, "err", err)
			}
			continue
		}
		if n > 0 {
			w.log.Info("embedded", "tier", tier.name, "count", n)
		}
	}
}

type embedRow struct {
	Key  string
	Text string
}

func (w *Worker) embedAtoms(ctx context.Context) (int, error) {
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, content FROM atoms
		WHERE embedding IS NULL AND content <> ''
		ORDER BY created_at LIMIT ?`, w.batchSize())
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	return w.scanAndEmbed(ctx, rows, "atoms")
}

func (w *Worker) embedScenes(ctx context.Context) (int, error) {
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, COALESCE(NULLIF(TRIM(abstract), ''), COALESCE(body, '')) AS text
		FROM scenes WHERE embedding IS NULL
		ORDER BY created_at LIMIT ?`, w.batchSize())
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	return w.scanAndEmbed(ctx, rows, "scenes")
}

func (w *Worker) embedMemories(ctx context.Context) (int, error) {
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, COALESCE(NULLIF(TRIM(abstract), ''), COALESCE(body, '')) AS text
		FROM memories WHERE embedding IS NULL AND superseded_at IS NULL
		ORDER BY created_at LIMIT ?`, w.batchSize())
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	return w.scanAndEmbed(ctx, rows, "memories")
}

func (w *Worker) scanAndEmbed(ctx context.Context, rows *sql.Rows, table string) (int, error) {
	var items []embedRow
	for rows.Next() {
		var r embedRow
		if err := rows.Scan(&r.Key, &r.Text); err != nil {
			return 0, err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return w.embedAndStore(ctx, table, items)
}

func (w *Worker) embedAndStore(ctx context.Context, table string, rows []embedRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	inputs := make([]string, len(rows))
	for i, r := range rows {
		text := r.Text
		if text == "" {
			text = "(empty)"
		}
		inputs[i] = text
	}

	vectors, err := w.llm.Embed(ctx, inputs)
	if err != nil {
		return 0, err
	}
	if len(vectors) != len(rows) {
		return 0, fmt.Errorf("embed returned %d vectors for %d rows", len(vectors), len(rows))
	}

	written := 0
	for i, r := range rows {
		blob, err := sqlite_vec.SerializeFloat32(vectors[i])
		if err != nil {
			return written, fmt.Errorf("serialize vector: %w", err)
		}
		_, err = w.db.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET embedding = ? WHERE id = ?`, table), blob, r.Key)
		if err != nil {
			return written, fmt.Errorf("update %s embedding: %w", table, err)
		}
		written++
	}
	return written, nil
}

func (w *Worker) batchSize() int {
	if w.cfg.EmbedBatchSize > 0 {
		return w.cfg.EmbedBatchSize
	}
	return 32
}
