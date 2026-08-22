The rmb CLI lives at `~/.rmb/bin/rmb`. Use this full path whenever `rmb` is not on PATH.

## Memory pyramid (T0 → T3)

| Tier | URI | What |
|------|-----|------|
| sessions | rmb://sessions/<id> | conversation container |
| turns | rmb://turns/<uuid> | raw user+assistant exchange |
| atoms | rmb://atoms/<uuid> | facts extracted from one session |
| scenes | rmb://scenes/<uuid> | per-session summary (evidence for memories) |
| memories | see below | long-term distilled facts (the index) |

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

1. `rmb search "<what they asked>"` — default scope is memory + skills (scenes are NOT included).
2. If a `[skills]` hit looks relevant, activate it before acting:
   - `rmb cat rmb://skills/<name>` — read SKILL.md and follow it
   - scripts: `rmb pull rmb://skills/<name>` → run from `~/.rmb/skills/<name>/`
3. Unsure what is available? `rmb ls rmb://skills/` — catalog of name + description.
4. Do not wing it when a skill matches — skills outrank your defaults for that task.

Narrow scope when you know the tier: `--scope=memory`, `--scope=scene`, or `--scope=skill`.

## Skills reference

| Tier | Command | Content |
|------|---------|---------|
| 1 Catalog | `rmb ls rmb://skills/` | name + description per skill |
| 2 Activation | `rmb cat rmb://skills/<name>` | full SKILL.md |
| 3 Resources | `rmb cat rmb://skills/<name>/<path>` | scripts, references, assets |

Local cache (for script execution): `rmb pull rmb://skills/<name>` → `~/.rmb/skills/<name>/`.
Pull all skills: `rmb pull rmb://skills/`. Push edits back: `rmb put rmb://skills/<name>` from `~/.rmb/skills/<name>/`.

## Choosing a query tool

| Question | Command | Ordering |
|----------|---------|----------|
| semantic lookup ("what is X", "how does Y work") | `rmb search "<query>"` | relevance only — no time factor |
| recent / recency intent ("what did I do this week") | `rmb search "<query>" --since=7d` | relevance, restricted to the time window (`--since`/`--until`: `2026-08-01`, `15:04`, or `7d`/`12h`/`30m`) |
| timeline / "what happened in June" | `rmb ls rmb://events/2026-06` | `updated_at DESC`, newest first (≤200 per page) |
| single item detail | `rmb cat <uri>` | full body |
| depth behind a memory (evidence scene) | `rmb meta <uri>` → `source_scene_uris`, then `rmb cat rmb://scenes/<uuid>` | full scene body |
| timestamps / version of one memory | `rmb meta <uri>` | created_at, updated_at |

How to read search output — each hit prints a fused score and version:

```
1. [memories] rmb://events/2026-08-21-openresty-dynamic-dns (0.0123, v=89)
    snippet… (+scene depth: rmb://scenes/<uuid>)
```

- The `(0.0123)` is the fused score. `v=89` is the version count: a HIGH version
  means the memory was heavily rewritten — treat it as possibly eroded and verify
  via its linked scene.
- `(+scene depth: rmb://scenes/…)` annotates a memory whose evidence scene was
  suppressed from the list — cat that scene for the underlying details.
- Never infer "latest" from search top-k: ranking ignores time entirely, so recent
  events may rank below older ones. For "what's new / recently", use `--since` or
  browse with ls, then cat the interesting uris.

## Tier routing (which tier holds the truth)

- **decisions / why / what happened** → events (`rmb://events/<slug>`, immutable)
  plus their linked scenes — the rationale often survives only in a scene.
- **stable facts** (people, projects, hosts, tools) → entities / preferences
  (`rmb://entities/<slug>`, `rmb://preferences/<slug>`, `rmb://profile`).
- **skills** outrank your defaults for that task when one is activated.
- Scenes are evidence, not a default search tier — reach them by drill-down
  (`rmb meta <memory>` → `source_scene_uris`, or the `(+scene depth: …)` annotation)
  or explicitly via `--scope=scene`.

## CLI rules

- Running `rmb` with no arguments prints profile and this guide from the local daemon.
- search "<query>" before asking the user (includes memory + skills by default), then cat / meta / ls as needed.
- search [--scope=memory,scene,skill] [--k=n] [--since=<date|7d>] [--until=<date|7d>] — only search accepts --scope; --since/--until restrict results to a time window.
- ls <uri-prefix> — list container contents: rmb://events/, rmb://scenes/, rmb://skills/, rmb://sessions/<id>/ (not rmb://memories/). The first segment is a prefix filter: `rmb ls rmb://events/2026-06` lists June events. Paginate with `--limit`/`--offset` (default 200 per page) and window with `--since`/`--until`/`--count`.
- Never invent uris.
- Recall is read-only. Workers distill new facts after conversations.
