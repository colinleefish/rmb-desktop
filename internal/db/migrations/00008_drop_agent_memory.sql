-- +goose Up
-- The rmb://agent guide is now served from the embedded bundle
-- (internal/agentmemory), not stored as a memory row. Purge any pre-existing
-- agent rows so they stop shadowing the bundled content in search/recall and
-- stop inflating the memories version counter.
--
-- The FTS index is external-content (content='memories', no triggers at this
-- schema version). A multi-row `DELETE FROM memories_fts WHERE rowid IN (...)`
-- on an external-content FTS5 table returns SQLITE_CORRUPT ("database disk
-- image is malformed") when two or more rows match — which blocked upgrades on
-- any v7 DB that had accumulated multiple rmb://agent memory versions. Instead
-- delete the base rows first, then rebuild the index from the remaining
-- (non-agent) content. This is safe whether or not agent rows exist.
DELETE FROM memories WHERE category = 'agent';

INSERT INTO memories_fts(memories_fts) VALUES('rebuild');

-- +goose Down
-- No-op: the agent guide is served from the embedded bundle; the memory row is
-- intentionally not restored.
