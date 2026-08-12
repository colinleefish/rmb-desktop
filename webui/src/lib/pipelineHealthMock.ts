import type { PipelineHealth } from "./types";

/** Story: user imported ~100 sessions; work is stuck / dropping off at T3. */
export function mockPipelineHealth(): PipelineHealth {
  return {
    distillation_enabled: true,
    tracked_sessions: 100,
    generated_at: new Date().toISOString(),
    stages: {
      t1: {
        pending: 5,
        running: 2,
        failed: 1,
        idle: 92,
        waiting: 0,
      },
      t2: {
        pending: 20,
        running: 1,
        failed: 1,
        idle: 70,
        waiting: 8,
      },
      t3: {
        pending: 58,
        running: 0,
        failed: 0,
        idle: 12,
        waiting: 30,
      },
    },
    funnel: {
      sessions: 100,
      t1_done: 92,
      t2_done: 70,
      t3_done: 12,
    },
    problems: [
      {
        session_key: "cursor:agent-abc123",
        session_uri: "rmb://sessions/cursor:agent-abc123",
        stage: "t1",
        status: "failed",
        updated_at: new Date(Date.now() - 45 * 60_000).toISOString(),
        reason: "LLM request failed: 401 unauthorized",
      },
      {
        session_key: "claude:proj-deploy-42",
        session_uri: "rmb://sessions/claude:proj-deploy-42",
        stage: "t2",
        status: "failed",
        updated_at: new Date(Date.now() - 30 * 60_000).toISOString(),
        reason: "scene parse returned empty body",
      },
      {
        session_key: "cursor:import-batch-017",
        session_uri: "rmb://sessions/cursor:import-batch-017",
        stage: "t3",
        status: "pending",
        updated_at: new Date(Date.now() - 90 * 60_000).toISOString(),
        reason: "waiting in L3 backlog",
      },
      {
        session_key: "cursor:import-batch-003",
        session_uri: "rmb://sessions/cursor:import-batch-003",
        stage: "t3",
        status: "pending",
        updated_at: new Date(Date.now() - 85 * 60_000).toISOString(),
        reason: "waiting in L3 backlog",
      },
      {
        session_key: "codex:refactor-hooks",
        session_uri: "rmb://sessions/codex:refactor-hooks",
        stage: "t1",
        status: "pending",
        updated_at: new Date(Date.now() - 20 * 60_000).toISOString(),
        reason: "gated by l1_every_n / idle window",
      },
    ],
  };
}
