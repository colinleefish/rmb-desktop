package recallstats_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/recallstats"
)

func TestRecordSearchCatMeta(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	svc := recallstats.NewService(database)
	ctx := context.Background()

	if err := svc.RecordSearch(ctx, []string{"rmb://entities/jenkins", "rmb://entities/jenkins"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordCat(ctx, "rmb://entities/jenkins"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordMeta(ctx, "rmb://entities/jenkins"); err != nil {
		t.Fatal(err)
	}

	stats, err := svc.BatchGet(ctx, []string{"rmb://entities/jenkins"})
	if err != nil {
		t.Fatal(err)
	}
	got := stats["rmb://entities/jenkins"]
	if got.SearchCount != 1 || got.CatCount != 1 || got.MetaCount != 1 {
		t.Fatalf("counts: search=%d cat=%d meta=%d", got.SearchCount, got.CatCount, got.MetaCount)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	return database
}
