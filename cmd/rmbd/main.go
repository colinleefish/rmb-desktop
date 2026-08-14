package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/debug"
	"github.com/colinleefish/rmb-desktop/internal/httpserver"
	"github.com/colinleefish/rmb-desktop/internal/launchatlogin"
	"github.com/colinleefish/rmb-desktop/internal/worker"
)

func main() {
	os.Exit(run())
}

func run() int {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"serve"}
	}

	switch args[0] {
	case "serve":
		return serve(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		return 2
	}
}

// dbPoolSize bounds the SQLite connection pool to the session-worker concurrency
// limits (L1 + L2, dynamic via config) plus the fixed embed/L3 workers and
// headroom for the local HTTP server. The pool is lazy, so this is a ceiling,
// not a preallocation.
func dbPoolSize(cfg config.Config) int {
	const fixedWorkers = 2 // embed + L3 (single goroutine each)
	const httpHeadroom = 8 // local HTTP server (upload + recall reads)
	return cfg.Pipeline.L1MaxConcurrency + cfg.Pipeline.L2MaxConcurrency + fixedWorkers + httpHeadroom
}

func serve(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config.yaml")
	_ = fs.Parse(args)

	cfgFile, err := config.ResolvePath(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config path: %v\n", err)
		return 1
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		return 1
	}
	defer database.Close()
	// Bound the pool to worker concurrency limits (dynamic via config) plus
	// headroom for the fixed embed/L3 workers and the local HTTP server.
	database.SetMaxOpenConns(dbPoolSize(cfg))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logBuf := debug.NewLogBuffer(2000)
	baseLog := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	log := debug.NewLogger(logBuf, baseLog)

	if cfg.LaunchAtLogin {
		if err := launchatlogin.Set(true); err != nil {
			log.Warn("launch at login sync failed", "err", err)
		}
	}

	reg := debug.NewRegistry()
	runner := worker.NewRunner(cfg, database, log, reg)
	runner.Start(ctx)
	defer runner.Wait()

	if err := httpserver.ListenAndServe(ctx, cfg.Addr, database, cfg, cfgFile, log, reg, logBuf); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}
