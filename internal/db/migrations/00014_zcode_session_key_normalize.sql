-- +goose Up
-- Issue #62: ZCode session keys were stored verbatim as ZCode's native ids
-- (sess_<uuid>) while every other agent stores bare UUIDs. The hook parser
-- now normalizes (zcodeRMBSessionID: strips sess_ when the remainder is a
-- valid UUID), so existing rows are rewritten to the same canonical form —
-- otherwise the next upload from the same ZCode session would land under a
-- new key and orphan the old session.
--
-- Only reducible keys are rewritten: sess_ + exactly one lowercase UUID
-- (GLOB matches the whole string, so the pattern pins the shape and length).
-- Non-reducible ids (e.g. subagent children sess_subagent_agent_<uuid>) are
-- left alone — old sessions stay reachable under their original key.
--
-- The NOT EXISTS guard keeps the rewrite collision-safe against the unique
-- index on session_key (target key already present → leave the row as is),
-- and makes the statement idempotent: normalized rows no longer match.
UPDATE sessions
SET session_key = substr(session_key, 6)
WHERE source = 'zcode'
  AND session_key GLOB 'sess_[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]-[0-9a-f][0-9a-f][0-9a-f][0-9a-f]-[0-9a-f][0-9a-f][0-9a-f][0-9a-f]-[0-9a-f][0-9a-f][0-9a-f][0-9a-f]-[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]'
  AND NOT EXISTS (
      SELECT 1 FROM sessions other
      WHERE other.session_key = substr(sessions.session_key, 6)
  );

-- +goose Down
-- Irreversible rewrite: the original sess_-prefixed keys cannot be
-- reconstructed from the normalized form. The Up statement is idempotent,
-- so re-applying it is always a no-op on already-normalized data.
SELECT 1;
