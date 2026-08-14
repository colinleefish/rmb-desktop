-- +goose Up
-- Adds the 'agent' category to the memories CHECK constraint. The rmb://agent
-- guide itself is served from the embedded bundle (internal/agentmemory), not
-- stored as a memory row; any pre-existing agent rows are purged by migration
-- 00008.
-- +goose StatementBegin
PRAGMA foreign_keys=OFF;

CREATE TABLE memories_new (
    id TEXT PRIMARY KEY,
    uri TEXT NOT NULL,
    category TEXT NOT NULL CHECK (
        category IN ('profile', 'preferences', 'entities', 'events', 'agent')
    ),
    slug TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    superseded_at INTEGER,
    abstract TEXT,
    body TEXT,
    source_scene_uris TEXT NOT NULL DEFAULT '[]',
    source_correction_uris TEXT NOT NULL DEFAULT '[]',
    embedding BLOB,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

INSERT INTO memories_new
SELECT * FROM memories;

DROP TABLE memories;

ALTER TABLE memories_new RENAME TO memories;

CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_active_uri
    ON memories (uri)
    WHERE superseded_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_memories_category ON memories (category);

DELETE FROM memories_fts;

INSERT INTO memories_fts (rowid, abstract, body)
SELECT rowid, COALESCE(abstract, ''), COALESCE(body, '') FROM memories;

PRAGMA foreign_keys=ON;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
PRAGMA foreign_keys=OFF;

DELETE FROM memories WHERE uri = 'rmb://agent';

CREATE TABLE memories_old (
    id TEXT PRIMARY KEY,
    uri TEXT NOT NULL,
    category TEXT NOT NULL CHECK (
        category IN ('profile', 'preferences', 'entities', 'events')
    ),
    slug TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    superseded_at INTEGER,
    abstract TEXT,
    body TEXT,
    source_scene_uris TEXT NOT NULL DEFAULT '[]',
    source_correction_uris TEXT NOT NULL DEFAULT '[]',
    embedding BLOB,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

INSERT INTO memories_old
SELECT * FROM memories;

DROP TABLE memories;

ALTER TABLE memories_old RENAME TO memories;

CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_active_uri
    ON memories (uri)
    WHERE superseded_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_memories_category ON memories (category);

DELETE FROM memories_fts;

INSERT INTO memories_fts (rowid, abstract, body)
SELECT rowid, COALESCE(abstract, ''), COALESCE(body, '') FROM memories;

PRAGMA foreign_keys=ON;
-- +goose StatementEnd
