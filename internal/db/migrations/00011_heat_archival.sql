-- +goose Up
-- P3.3 heat-based archival (issue #32, plan §10 / D2=90d).
--
-- Cold memories are ARCHIVED: taken out of default search (they are still
-- cat-able by direct uri and restorable in one command). Nothing auto-deletes.
--
-- Applies to the memories table ONLY. Evidence tiers (turns/atoms/scenes/
-- skills) are exempt forever (§9.2 recoverable-evidence invariant).
ALTER TABLE memories ADD COLUMN archived_at INTEGER;

-- Fast path for the recall memory-tier filter (archived_at IS NULL):
CREATE INDEX IF NOT EXISTS idx_memories_not_archived
    ON memories (updated_at)
    WHERE superseded_at IS NULL AND archived_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_memories_not_archived;
ALTER TABLE memories DROP COLUMN archived_at;
