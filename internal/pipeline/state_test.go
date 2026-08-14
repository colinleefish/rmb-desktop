package pipeline_test

import (
	"context"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/pipeline"
	"github.com/google/uuid"
)

func TestResetRunningOlderThan(t *testing.T) {
	path := t.TempDir() + "/test.db"
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	sessionID := uuid.NewString()
	nowMS := time.Now().UTC().UnixMilli()
	oldMS := nowMS - int64(10*time.Minute/time.Millisecond)

	if _, err := database.Exec(`
		INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES (?, 'stuck', ?, ?)`,
		sessionID, nowMS, nowMS); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO pipeline_state (
			session_id, l1_status, l2_status, l3_status,
			l2_started_at, updated_at
		) VALUES (?, 'idle', 'running', 'idle', ?, ?)`,
		sessionID, oldMS, nowMS); err != nil {
		t.Fatal(err)
	}

	n, err := pipeline.ResetRunningOlderThan(context.Background(), database, pipeline.StageL2, nowMS-int64(5*time.Minute/time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 reset, got %d", n)
	}

	var status string
	if err := database.QueryRow(`SELECT l2_status FROM pipeline_state WHERE session_id = ?`, sessionID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("expected pending, got %s", status)
	}
}
