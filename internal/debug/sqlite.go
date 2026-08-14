package debug

import (
	"context"
	"database/sql"
	"fmt"
)

// SQLiteStats is returned by GET /api/v1/debug/sqlite.
type SQLiteStats struct {
	Version       string `json:"version"`
	JournalMode   string `json:"journal_mode"`
	PageCount     int64  `json:"page_count"`
	PageSize      int64  `json:"page_size"`
	WALPages      int64  `json:"wal_pages,omitempty"`
	MaxOpenConns  int    `json:"max_open_conns"`
	OpenConns     int    `json:"open_conns"`
	InUse         int    `json:"in_use"`
	Idle          int    `json:"idle"`
}

// SQLiteStats collects SQLite and pool diagnostics.
func CollectSQLiteStats(ctx context.Context, database *sql.DB) (SQLiteStats, error) {
	var out SQLiteStats
	if err := database.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&out.Version); err != nil {
		return SQLiteStats{}, fmt.Errorf("sqlite version: %w", err)
	}
	if err := database.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&out.JournalMode); err != nil {
		return SQLiteStats{}, fmt.Errorf("journal mode: %w", err)
	}
	if err := database.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&out.PageCount); err != nil {
		return SQLiteStats{}, fmt.Errorf("page count: %w", err)
	}
	if err := database.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&out.PageSize); err != nil {
		return SQLiteStats{}, fmt.Errorf("page size: %w", err)
	}
	_ = database.QueryRowContext(ctx, `PRAGMA wal_checkpoint`).Scan()
	if err := database.QueryRowContext(ctx, `PRAGMA wal_pages`).Scan(&out.WALPages); err != nil {
		// wal_pages may be unavailable depending on build; ignore.
		out.WALPages = 0
	}
	stats := database.Stats()
	out.MaxOpenConns = stats.MaxOpenConnections
	out.OpenConns = stats.OpenConnections
	out.InUse = stats.InUse
	out.Idle = stats.Idle
	return out, nil
}
