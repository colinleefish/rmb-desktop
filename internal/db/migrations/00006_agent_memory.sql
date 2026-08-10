-- +goose Up
-- Curated agent guide at rmb://agent (singleton; not T3-distilled).
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

INSERT INTO memories (
    id, uri, category, slug, version, abstract, body,
    source_scene_uris, source_correction_uris, created_at, updated_at
)
SELECT
    '00000000-0000-4000-8000-000000000018',
    'rmb://agent',
    'agent',
    NULL,
    1,
    'Agent recall guide',
    '## Memory pyramid (T0 → T3)

| Tier | URI | What |
|------|-----|------|
| sessions | rmb://sessions/<id> | conversation container |
| turns | rmb://turns/<uuid> | raw user+assistant exchange |
| atoms | rmb://atoms/<uuid> | facts extracted from one session |
| scenes | rmb://scenes/<uuid> | per-session summary |
| memories | see below | long-term distilled facts |

## Memory uris

profile | entities/<slug> | preferences/<slug> | events/<slug> | scenes/<uuid> | skills/<name>

## Memory categories (T3)

| Category | URI | Content |
|----------|-----|--------|
| profile | rmb://profile | singleton — who the user is |
| agent | rmb://agent | singleton — how to use rmb (this doc) |
| preferences | rmb://preferences/<slug> | how the user wants AI to behave |
| entities | rmb://entities/<slug> | people, projects, hosts, tools |
| events | rmb://events/<slug> | dated decisions (immutable) |

## Skill auto-discovery

Skills are curated playbooks at `rmb://skills/<name>` — check them before improvising.

When the user asks you to do something (deploy, SSH, PDF, etc.):

1. `rmb search "<what they asked>"` — default scope includes memory, scene, and skills.
2. If a `[skills]` hit looks relevant, activate it before acting:
   - `rmb cat rmb://skills/<name>` — read SKILL.md and follow it
   - scripts: `rmb skill pull <name>` → run from `~/.rmb/skills/<name>/`
3. Unsure what is available? `rmb tree rmb://skills/` — catalog of name + description.
4. Do not wing it when a skill matches — skills outrank your defaults for that task.

Narrow scope when you know the tier: `--scope=memory`, `--scope=scene`, or `--scope=skill`.

## Skills reference

| Tier | Command | Content |
|------|---------|---------|
| 1 Catalog | `rmb tree rmb://skills/` | name + description per skill |
| 2 Activation | `rmb cat rmb://skills/<name>` | full SKILL.md |
| 3 Resources | `rmb cat rmb://skills/<name>/<path>` | scripts, references, assets |

Local cache (for script execution): `rmb skill pull <name>` → `~/.rmb/skills/<name>/`.
Push edits back: `rmb skill put <name>` from `~/.rmb/skills/<name>/`.

## CLI rules

- Running `rmb` with no arguments prints profile and this guide from the local daemon.
- search "<query>" before asking the user (includes skills by default), then cat / meta / tree as needed.
- search [--scope=...] — only search accepts --scope. cat/tree/meta take a single uri.
- tree <uri-prefix> — browse rmb://entities/, rmb://skills/, rmb://profile (not rmb://memories/).
- Never invent uris.
- Recall is read-only. Workers distill new facts after conversations.',
    '[]',
    '[]',
    CAST((strftime('%s', 'now') * 1000) AS INTEGER),
    CAST((strftime('%s', 'now') * 1000) AS INTEGER)
WHERE NOT EXISTS (
    SELECT 1 FROM memories WHERE uri = 'rmb://agent' AND superseded_at IS NULL
);

INSERT INTO memories_fts (rowid, abstract, body)
SELECT rowid, COALESCE(abstract, ''), COALESCE(body, '')
FROM memories
WHERE uri = 'rmb://agent' AND superseded_at IS NULL
  AND rowid NOT IN (SELECT rowid FROM memories_fts);

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
