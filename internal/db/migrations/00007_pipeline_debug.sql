-- +goose Up
ALTER TABLE pipeline_state ADD COLUMN l1_last_error TEXT;
ALTER TABLE pipeline_state ADD COLUMN l2_last_error TEXT;
ALTER TABLE pipeline_state ADD COLUMN l3_last_error TEXT;
ALTER TABLE pipeline_state ADD COLUMN l1_started_at INTEGER;
ALTER TABLE pipeline_state ADD COLUMN l2_started_at INTEGER;
ALTER TABLE pipeline_state ADD COLUMN l3_started_at INTEGER;

-- +goose Down
ALTER TABLE pipeline_state DROP COLUMN l3_started_at;
ALTER TABLE pipeline_state DROP COLUMN l2_started_at;
ALTER TABLE pipeline_state DROP COLUMN l1_started_at;
ALTER TABLE pipeline_state DROP COLUMN l3_last_error;
ALTER TABLE pipeline_state DROP COLUMN l2_last_error;
ALTER TABLE pipeline_state DROP COLUMN l1_last_error;
