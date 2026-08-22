-- +goose Up
-- Usage-heat (plan §10, issue #24): heat decays toward zero and only
-- qualifying *use* (cat/meta) adds weight. Bare search impressions never
-- update heat — exposure must not self-reinforce ranking.
ALTER TABLE recall_stats ADD COLUMN heat REAL NOT NULL DEFAULT 0;
ALTER TABLE recall_stats ADD COLUMN last_use_at INTEGER;

-- Local-only query telemetry: every search with its top-k, plus the
-- search→cat join (catted_uri/catted_at set when a cat of a top-k uri
-- follows within the 10-minute window). Feeds doctor metrics (zero-cat
-- search rate, heat concentration) and later ranking calibration (P1.3).
CREATE TABLE IF NOT EXISTS search_queries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    query      TEXT NOT NULL,
    scope      TEXT,
    k          INTEGER NOT NULL,
    top_uris   TEXT NOT NULL,          -- JSON array of ranked URIs
    ts         INTEGER NOT NULL,       -- ms epoch, search time
    catted_uri TEXT,                   -- first uri from top-k that got a cat within the window
    catted_at  INTEGER                 -- ms epoch of that cat
);

CREATE INDEX IF NOT EXISTS idx_search_queries_ts ON search_queries (ts);

-- +goose Down
DROP TABLE IF EXISTS search_queries;
ALTER TABLE recall_stats DROP COLUMN last_use_at;
ALTER TABLE recall_stats DROP COLUMN heat;
