The rmb CLI lives at `~/.rmb/bin/rmb`. Use this full path whenever `rmb` is not on PATH.

## Memory pyramid (T0 → T3)

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

## Choosing a query tool

| Question | Command | Ordering |
|----------|---------|----------|
| semantic lookup ("what is X", "how does Y work") | `rmb search "<query>"` | relevance only — no time factor |
| latest / timeline / "what happened recently" | `rmb tree rmb://events/` (scenes: `rmb://scenes/`) | `updated_at DESC`, newest first (≤200) |
| single item detail | `rmb cat <uri>` | full body |
| timestamps / version of one memory | `rmb meta <uri>` | created_at, updated_at |

Never infer "latest" from search top-k: ranking ignores time entirely, so recent events
may rank below older ones. For "what's new / recently", browse with tree, then cat the
interesting uris.

## CLI rules

- Running `rmb` with no arguments prints profile and this guide from the local daemon.
- search "<query>" before asking the user (includes skills by default), then cat / meta / tree as needed.
- search [--scope=...] — only search accepts --scope. cat/tree/meta take a single uri.
- tree <uri-prefix> — browse rmb://entities/, rmb://skills/, rmb://profile (not rmb://memories/).
- Never invent uris.
- Recall is read-only. Workers distill new facts after conversations.
