-- +goose Up
ALTER TABLE sessions ADD COLUMN source TEXT;

CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions (source);

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_source;
