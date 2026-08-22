package worker_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/db"
	"github.com/colinleefish/rmb-desktop/internal/llm"
	"github.com/colinleefish/rmb-desktop/internal/model"
	"github.com/colinleefish/rmb-desktop/internal/worker/extract"
	"github.com/colinleefish/rmb-desktop/internal/workerlock"
	"github.com/google/uuid"
)

type mockLLM struct{}

func (m *mockLLM) ExtractAtoms(_ context.Context, _ string) (string, error) {
	return `{"atoms":[{"category":"profile","priority":80,"scene_name":"Identity","content":"User name is Colin","source_turn_indices":[0]}]}`, nil
}

func (m *mockLLM) BuildScenes(_ context.Context, _ string) (string, error) {
	return `{"scenes":[]}`, nil
}

func (m *mockLLM) SummarizeSessionAbstract(_ context.Context, _ string) (string, error) {
	return "Session about identity", nil
}

func (m *mockLLM) DistillMemory(_ context.Context, _, _, _ string, _ []string, _ []llm.RelatedEvent) (string, error) {
	return `{"abstract":"Profile","body":"Colin is the user."}`, nil
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func TestExtractOneCycle(t *testing.T) {
	database := openTestDB(t)
	defer database.Close()

	sessionID := uuid.NewString()
	nowMS := time.Now().UTC().UnixMilli()
	mustExec(t, database, `INSERT INTO sessions (id, session_key, created_at, updated_at) VALUES (?, 'pipe-test', ?, ?)`, sessionID, nowMS, nowMS)
	mustExec(t, database, `
		INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, warmup_threshold, updated_at)
		VALUES (?, 'pending', 'idle', 'idle', 2, ?)`, sessionID, nowMS)

	msgs, _ := json.Marshal([]map[string]string{{"role": "user", "content": "I am Colin"}})
	turnID := uuid.NewString()
	mustExec(t, database, `
		INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
		VALUES (?, ?, ?, ?, 'pending')`, turnID, sessionID, string(msgs), nowMS)

	cfg, _ := config.Default()
	cfg.Pipeline.L1EveryN = 1
	cfg.Pipeline.L1Warmup = false

	locks := workerlock.NewSessionLocks()
	w := extract.NewWorker(database, &mockLLM{}, cfg.Pipeline, locks, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { _ = w.Run(ctx) }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		_ = database.QueryRow(`SELECT COUNT(*) FROM atoms WHERE session_id = ?`, sessionID).Scan(&count)
		if count > 0 {
			cancel()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM atoms WHERE session_id = ?`, sessionID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("expected atoms after L1 extraction")
	}

	var l2Status string
	if err := database.QueryRow(`SELECT l2_status FROM pipeline_state WHERE session_id = ?`, sessionID).Scan(&l2Status); err != nil {
		t.Fatal(err)
	}
	if l2Status != model.PipelineStatusPending {
		t.Fatalf("expected l2 pending, got %s", l2Status)
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}
