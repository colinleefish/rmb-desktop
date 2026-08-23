-- +goose Up
-- P3.1 true event times (issue #30, plan §5 P3.1 / audit RC6, RC5).
--
-- Events were dated only by slug convention; backfills carry write-time
-- timestamps, so the timeline read as if the 08-13 backfill batch happened
-- in August though the decisions were made in June/July. occurred_at stores
-- WHEN the event actually happened, backfilled here from the slug date
-- prefix (the body-date fallback runs in Go at startup for rows the slug
-- can't date), and set at distill time for new events.
ALTER TABLE memories ADD COLUMN occurred_at INTEGER;

-- Backfill from slug dates: YYYY-MM-DD- prefix -> UTC midnight, in ms.
-- Invalid dates under strftime yield NULL and stay for the Go fallback.
UPDATE memories
SET occurred_at = CAST(strftime('%s', substr(slug, 1, 10)) AS INTEGER) * 1000
WHERE category = 'events'
  AND occurred_at IS NULL
  AND slug GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-*';

-- Events ls/--since default ordering (occurred_at DESC).
CREATE INDEX IF NOT EXISTS idx_memories_events_occurred
    ON memories (occurred_at)
    WHERE category = 'events' AND superseded_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_memories_events_occurred;
ALTER TABLE memories DROP COLUMN occurred_at;
