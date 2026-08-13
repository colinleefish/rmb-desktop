package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/colinleefish/rmb-desktop/internal/agentmemory"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Open opens SQLite at path, creates parent dirs, runs migrations, enables WAL.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	// busy_timeout makes concurrent writers wait up to 5s for the write lock
	// instead of immediately failing with SQLITE_BUSY ("database is locked").
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Connection pool is left unbounded here. database/sql opens connections
	// lazily, so the live connection count already tracks actual concurrency; WAL
	// allows concurrent readers + one writer. Callers that want a ceiling (e.g.
	// to follow worker concurrency limits) should call db.SetMaxOpenConns.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := agentmemory.UpsertAgentGuide(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed agent guide: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// VecVersion returns sqlite-vec version string, if loaded.
func VecVersion(db *sql.DB) (string, error) {
	var v string
	err := db.QueryRow("SELECT vec_version()").Scan(&v)
	if err != nil {
		return "", err
	}
	return v, nil
}

// SQLiteVersion returns the SQLite library version.
func SQLiteVersion(db *sql.DB) (string, error) {
	var v string
	err := db.QueryRow("SELECT sqlite_version()").Scan(&v)
	if err != nil {
		return "", err
	}
	return v, nil
}
