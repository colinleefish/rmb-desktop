-- +goose Up
-- Keep scenes_fts (external-content FTS5) in sync with scenes via triggers.
--
-- Previously the L2 scene worker managed scenes_fts by hand, issuing an FTS5
-- 'delete' command followed by an insert for every upserted scene. That had two
-- problems specific to external-content FTS5 tables:
--
--   1. The 'delete' command requires the column values to be supplied explicitly
--      (it does NOT read the content table). The old code passed only the rowid,
--      so the delete was a silent no-op on updates, leaving stale terms behind.
--   2. On first-time scene creation the rowid exists in scenes but was never
--      indexed; deleting it makes FTS5 return SQLITE_CORRUPT ("database disk
--      image is malformed"), aborting the whole persist transaction so no scene
--      was ever written.
--
-- Triggers solve both: AFTER INSERT/UPDATE/DELETE keep the index in sync with
-- normal INSERT/UPDATE/DELETE on scenes, so the worker no longer touches FTS.
-- The UPDATE trigger is gated on abstract/body actually changing, so embedding
-- writes (UPDATE scenes SET embedding=...) do not needlessly reindex the row.

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS scenes_ai AFTER INSERT ON scenes BEGIN
    INSERT INTO scenes_fts(rowid, abstract, body) VALUES (new.rowid, new.abstract, new.body);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS scenes_ad AFTER DELETE ON scenes BEGIN
    INSERT INTO scenes_fts(scenes_fts, rowid, abstract, body) VALUES ('delete', old.rowid, old.abstract, old.body);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS scenes_au AFTER UPDATE ON scenes FOR EACH ROW WHEN
    old.abstract IS NOT new.abstract OR old.body IS NOT new.body
BEGIN
    INSERT INTO scenes_fts(scenes_fts, rowid, abstract, body) VALUES ('delete', old.rowid, old.abstract, old.body);
    INSERT INTO scenes_fts(rowid, abstract, body) VALUES (new.rowid, new.abstract, new.body);
END;
-- +goose StatementEnd
