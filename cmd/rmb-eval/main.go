// Command rmb-eval builds golden fixtures from a read-only store and runs the
// offline recall evaluation harness.
//
//	rmb-eval snapshot -db <store.db> -golden golden.yaml -out fixture.json
//	rmb-eval run -fixture fixture.json -golden golden.yaml [--print-metrics]
//
// snapshot reads the store READ-ONLY (never writes, never touches the daemon).
// run builds a scratch rmb.db in a temp dir and evaluates recall in-process
// with a deterministic hash embedder, so it works offline in CI.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/colinleefish/rmb-desktop/internal/recall/eval"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "snapshot":
		err = cmdSnapshot(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "rmb-eval:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  rmb-eval snapshot -db <store.db> -golden <golden.yaml> -out <fixture.json>
  rmb-eval run -fixture <fixture.json> -golden <golden.yaml>
snapshot reads the store read-only. run is offline and deterministic.`)
}

func cmdSnapshot(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	dbPath := fs.String("db", "", "path to store .db (read-only access)")
	goldenPath := fs.String("golden", "internal/recall/eval/golden.yaml", "golden.yaml")
	outPath := fs.String("out", "internal/recall/eval/testdata/golden_fixture.json", "output fixture")
	stride := fs.Int("stride", 7, "keep every Nth memory row as a distractor")
	sceneStride := fs.Int("scene-stride", 5, "keep every Nth scene row as a distractor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dbPath == "" {
		return fmt.Errorf("-db is required")
	}
	golden, err := eval.LoadGolden(*goldenPath)
	if err != nil {
		return err
	}

	// READ-ONLY connection. Never write to the source.
	dsn := "file:" + *dbPath + "?mode=ro"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return err
	}

	expected := map[string]bool{}
	for _, q := range golden.Questions {
		for _, u := range q.Expected {
			expected[u] = true
		}
	}

	fix := eval.Fixture{Version: 1, Source: filepath.Base(*dbPath)}

	if err := snapshotMemories(db, expected, *stride, &fix); err != nil {
		return err
	}
	if err := snapshotScenes(db, expected, *sceneStride, &fix); err != nil {
		return err
	}
	if err := snapshotSkills(db, &fix); err != nil {
		return err
	}

	data, err := json.MarshalIndent(&fix, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(*outPath, data, 0o644)
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fixturePath := fs.String("fixture", "internal/recall/eval/testdata/golden_fixture.json", "fixture.json")
	goldenPath := fs.String("golden", "internal/recall/eval/golden.yaml", "golden.yaml")
	keepDB := fs.Bool("keep-db", false, "keep scratch db path (debug)")
	printReport := fs.Bool("print-report", false, "print per-question JSON report")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Silence goose migration logs so the eval output stays clean.
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	fix, err := eval.LoadFixture(*fixturePath)
	if err != nil {
		return err
	}
	golden, err := eval.LoadGolden(*goldenPath)
	if err != nil {
		return err
	}

	scratch := filepath.Join(os.TempDir(), "rmb-eval", "scratch.db")
	if *keepDB {
		_ = os.MkdirAll(filepath.Dir(scratch), 0o755)
		_ = os.Remove(scratch)
	} else {
		scratch = filepath.Join(os.TempDir(), "rmb-eval-"+fmt.Sprint(os.Getpid()), "scratch.db")
		_ = os.MkdirAll(filepath.Dir(scratch), 0o755)
		defer os.RemoveAll(filepath.Dir(scratch))
	}
	database, err := fix.BuildDB(scratch)
	if err != nil {
		return err
	}
	defer database.Close()

	report, err := eval.Run(context.Background(), database, golden)
	if err != nil {
		return err
	}

	fmt.Printf("recall@5=%.3f  dup-rate(top5)=%.3f  recency-precision=%.3f  (%d/%d questions hit)\n",
		report.RecallAt5, report.DupRate, report.RecencyPrecision, report.Passed, report.TotalQuestions)
	if *printReport {
		for _, r := range report.Questions {
			fmt.Printf("%-30s hit=%-5v top5=%v\n", r.ID, r.Hit, r.Top5)
		}
	}
	fmt.Println("--- gates ---")
	ok := report.GatesOK(&golden.Gates)
	fmt.Printf("min_recall_at_5=%.2f  max_dup_rate=%.2f  min_recency_precision=%.2f  => %v\n",
		golden.Gates.MinRecallAt5, golden.Gates.MaxDupRate, golden.Gates.MinRecencyPrecision, ok)

	if !ok {
		return fmt.Errorf("eval gates not met")
	}
	return nil
}
