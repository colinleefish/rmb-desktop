-- +goose Up
CREATE TABLE IF NOT EXISTS recall_stats (
    uri              TEXT PRIMARY KEY,
    search_count     INTEGER NOT NULL DEFAULT 0,
    cat_count        INTEGER NOT NULL DEFAULT 0,
    meta_count       INTEGER NOT NULL DEFAULT 0,
    last_searched_at INTEGER,
    last_cated_at    INTEGER,
    last_metaed_at   INTEGER,
    updated_at       INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_recall_stats_search_count ON recall_stats (search_count DESC);
CREATE INDEX IF NOT EXISTS idx_recall_stats_cat_count ON recall_stats (cat_count DESC);

-- +goose Down
DROP TABLE IF EXISTS recall_stats;
