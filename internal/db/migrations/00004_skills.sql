-- +goose Up
-- Agent Skills bundles at rmb://skills/<name> (see agentskills.io specification).
CREATE TABLE IF NOT EXISTS skills (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL,
    uri TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    superseded_at INTEGER,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '[]',
    bundle_sha256 TEXT NOT NULL,
    fts_text TEXT NOT NULL DEFAULT '',
    embedding BLOB,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_slug_active
    ON skills (slug)
    WHERE superseded_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_uri_active
    ON skills (uri)
    WHERE superseded_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_skills_updated ON skills (updated_at);

CREATE TABLE IF NOT EXISTS skill_files (
    id TEXT PRIMARY KEY,
    skill_id TEXT NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    rel_path TEXT NOT NULL,
    content TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    content_sha256 TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE (skill_id, rel_path)
);

CREATE INDEX IF NOT EXISTS idx_skill_files_skill_id ON skill_files (skill_id);

CREATE VIRTUAL TABLE IF NOT EXISTS skills_fts USING fts5 (
    fts_text,
    content='skills',
    content_rowid='rowid',
    tokenize='unicode61'
);

-- +goose Down
DROP TABLE IF EXISTS skills_fts;
DROP TABLE IF EXISTS skill_files;
DROP TABLE IF EXISTS skills;
