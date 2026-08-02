-- +goose Up
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    session_key TEXT NOT NULL,
    abstract TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_session_key ON sessions (session_key);

CREATE TABLE IF NOT EXISTS session_turns (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    messages_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    l1_status TEXT NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_session_turns_session_created
    ON session_turns (session_id, created_at);

CREATE TABLE IF NOT EXISTS pipeline_state (
    session_id TEXT PRIMARY KEY REFERENCES sessions (id) ON DELETE CASCADE,
    l1_status TEXT NOT NULL DEFAULT 'idle',
    l2_status TEXT NOT NULL DEFAULT 'idle',
    l3_status TEXT NOT NULL DEFAULT 'idle',
    updated_at INTEGER NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS pipeline_state;
DROP TABLE IF EXISTS session_turns;
DROP TABLE IF EXISTS sessions;
