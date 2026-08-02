-- +goose Up
ALTER TABLE session_turns ADD COLUMN l1_extracted_at INTEGER;

ALTER TABLE pipeline_state ADD COLUMN l1_advanced_at INTEGER;
ALTER TABLE pipeline_state ADD COLUMN l2_advanced_at INTEGER;
ALTER TABLE pipeline_state ADD COLUMN l3_advanced_at INTEGER;
ALTER TABLE pipeline_state ADD COLUMN l1_turns_since_advanced INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pipeline_state ADD COLUMN warmup_threshold INTEGER NOT NULL DEFAULT 2;

CREATE TABLE IF NOT EXISTS atoms (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    category TEXT NOT NULL CHECK (
        category IN ('profile', 'preferences', 'entities', 'events')
    ),
    priority INTEGER NOT NULL DEFAULT 50,
    scene_name TEXT,
    slug TEXT,
    content TEXT NOT NULL,
    source_turn_ids TEXT NOT NULL DEFAULT '[]',
    embedding BLOB,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_atoms_session_id ON atoms (session_id);
CREATE INDEX IF NOT EXISTS idx_atoms_category ON atoms (category);

CREATE VIRTUAL TABLE IF NOT EXISTS atoms_fts USING fts5 (
    content,
    content='atoms',
    content_rowid='rowid',
    tokenize='unicode61'
);

CREATE TABLE IF NOT EXISTS scenes (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    display_name TEXT,
    abstract TEXT,
    body TEXT,
    source_atoms TEXT NOT NULL DEFAULT '[]',
    embedding BLOB,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scenes_session_id ON scenes (session_id);

CREATE VIRTUAL TABLE IF NOT EXISTS scenes_fts USING fts5 (
    abstract,
    body,
    content='scenes',
    content_rowid='rowid',
    tokenize='unicode61'
);

CREATE TABLE IF NOT EXISTS memories (
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_active_uri
    ON memories (uri)
    WHERE superseded_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_memories_category ON memories (category);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5 (
    abstract,
    body,
    content='memories',
    content_rowid='rowid',
    tokenize='unicode61'
);

CREATE TABLE IF NOT EXISTS corrections (
    id TEXT PRIMARY KEY,
    target_uris TEXT NOT NULL DEFAULT '[]',
    statement TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    retracted_at INTEGER
);

-- +goose Down
DROP TABLE IF EXISTS corrections;
DROP TABLE IF EXISTS memories_fts;
DROP TABLE IF EXISTS memories;
DROP TABLE IF EXISTS scenes_fts;
DROP TABLE IF EXISTS scenes;
DROP TABLE IF EXISTS atoms_fts;
DROP TABLE IF EXISTS atoms;
