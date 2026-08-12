package browse_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/browse"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/google/uuid"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func mustExec(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatalf("exec: %v\nquery: %s", err, query)
	}
}

func TestPipelineHealth(t *testing.T) {
	database := openTestDB(t)
	nowMS := time.Now().UTC().UnixMilli()

	insert := func(key, l1, l2, l3 string) {
		t.Helper()
		id := uuid.NewString()
		mustExec(t, database, `INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			id, key, nowMS, nowMS)
		mustExec(t, database, `
			INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, updated_at)
			VALUES (?, ?, ?, ?, ?)`, id, l1, l2, l3, nowMS)
	}

	insert("s-ok", "idle", "idle", "idle")
	insert("s-t1-fail", "failed", "idle", "idle")
	insert("s-t3-pending", "idle", "idle", "pending")
	insert("s-t1-run", "running", "idle", "idle")

	svc := browse.NewService(database, nil)
	health, err := svc.PipelineHealth(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}

	if !health.DistillationEnabled {
		t.Fatal("expected distillation enabled")
	}
	if health.TrackedSessions != 4 {
		t.Fatalf("tracked=%d want 4", health.TrackedSessions)
	}
	if health.Stages.T1.Failed != 1 || health.Stages.T1.Running != 1 {
		t.Fatalf("t1 counts unexpected: %+v", health.Stages.T1)
	}
	if health.Stages.T1.Waiting < 1 {
		t.Fatalf("expected t1 waiting without advanced_at: %+v", health.Stages.T1)
	}
	if health.Funnel.Sessions != 4 {
		t.Fatalf("funnel sessions=%d", health.Funnel.Sessions)
	}
	// advanced_at is unset in this fixture, so funnel done stays 0.
	if health.Funnel.T1Done != 0 || health.Funnel.T2Done != 0 || health.Funnel.T3Done != 0 {
		t.Fatalf("funnel without advanced_at should be 0: %+v", health.Funnel)
	}
	if health.Stages.T2.Waiting < 1 {
		t.Fatalf("expected t2 waiting for untouched defaults: %+v", health.Stages.T2)
	}
	if len(health.Problems) < 2 {
		t.Fatalf("problems=%d want at least failed + pending", len(health.Problems))
	}
	if health.Problems[0].Status != "failed" {
		t.Fatalf("first problem should be failed, got %+v", health.Problems[0])
	}
}
