-- +goose Up
-- The rmb://agent guide is now served from the embedded bundle
-- (internal/agentmemory), not stored as a memory row. Purge any pre-existing
-- agent rows so they stop shadowing the bundled content in search/recall and
-- stop inflating the memories version counter. The FTS index is external-content
-- (no triggers), so its entries must be removed explicitly before the base rows.
-- +goose StatementBegin
DELETE FROM memories_fts
WHERE rowid IN (SELECT rowid FROM memories WHERE category = 'agent');

DELETE FROM memories WHERE category = 'agent';
-- +goose StatementEnd

-- +goose Down
-- No-op: the agent guide is served from the embedded bundle; the memory row is
-- intentionally not restored.
