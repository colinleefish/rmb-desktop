-- +goose Up
-- P2.1 consolidation gates (issue #27, plan §9.3).
--
-- source_atom_hash records which atom IDs the active memory's body
-- incorporated at distill time. Atoms are append-only, so the hash is a
-- precise "evidence so far" fingerprint: the materiality gate skips
-- re-distillation when the bucket's atom set is unchanged (previously the
-- only gate was source-scene equality, which re-distilled the profile
-- ~8x/day on paraphrase atoms from every new session -- 166 versions).
--
-- idx_atoms_category_slug backs the L1 atom-level near-dup suppression
-- lookup (same-subject paraphrase atoms are skipped at ingest).
ALTER TABLE memories ADD COLUMN source_atom_hash TEXT;

CREATE INDEX IF NOT EXISTS idx_atoms_category_slug ON atoms(category, slug);

-- +goose Down
DROP INDEX IF EXISTS idx_atoms_category_slug;
ALTER TABLE memories DROP COLUMN source_atom_hash;
