# Audit report — sub-agent `time-traveler`: TIMELINE & RECENCY tests

**System**: rmb long-term memory (`~/.rmb/bin/rmb`, local daemon `rmbd` at `http://127.0.0.1:19019`, SQLite 3.53.4 + cgo-vec per `/healthz`)
**Audit date**: 2026-08-22 (daemon local time ≈ 14:26–14:37 UTC+8)
**Mode**: strictly read-only (`ls` / `cat` / `meta` / `search` / GET-curl only; nothing under `~/.rmb` touched)

---

## Method

1. `~/.rmb/bin/rmb --help` to learn the CLI contract (agent guide documents: events listed by `updated_at DESC`, ≤200, "immutable dated decisions"; search "ranking ignores time entirely").
2. Pulled the full `ls rmb://events/` output (exactly 200 rows) and ran `rmb meta` on **every one of the 200** events (200 calls, ~16 ms each) to machine-check ordering, `created_at` vs `updated_at`, versions, and `source_scene_uris`.
3. Probed beyond the 200 cap three ways: (a) two disjoint broad `rmb search --scope=memory --k=1000` queries, unioned their unique event uris; (b) date-prefix `ls` scoping attempts; (c) HTTP GETs against the daemon (`/`, `/healthz`, `/ui/`, guessed `/ls`-style endpoints) plus grep of the UI JS bundle (`/ui/assets/index-BCYhloao.js`) for hidden API routes.
4. Same cap probe for scenes (`search --scope=scene --k=1000`) and sessions (`ls rmb://sessions/`).
5. Cross-mapped events ↔ scenes via `source_scene_uris`, and spot-checked scene metas (`session_id`, `created_at`).
6. Ran the user-level test: `rmb search "recent work" --k=10` vs `ls rmb://events/ | head`, then `meta` on every hit to date it.

All commands returned sub-second (ls: 10–25 ms; meta: ~16 ms; search: 150–180 ms even at k=1000). The daemon is fast; the problems are all design-level, not latency.

---

## Evidence

### 1. Ordering of `ls rmb://events/` (updated_at DESC)

- `time ~/.rmb/bin/rmb ls rmb://events/` → 200 rows, **real 0.025s**. Saved to `/tmp/events_all.txt`.
- Full sweep: `rmb meta` on all 200 uris → `/tmp/events_meta.tsv` (200 rows, a ~5 s bash loop):
  - **Inversions: 0.** `updated_at` strictly non-increasing from `1787379990002` (2026-08-22T14:26:30, `rmb://events/rmb-desktop-release-0-2-8-dev-5`) down to `1787050361450` (2026-08-18T18:52:41, `rmb://events/2026-08-18-starlink-bbc-gunicorn-chart`).
  - The list is ordered by `updated_at`, **not** by slug date: e.g. rows 15–17 are `2026-06-13-*` events sandwiched between `2026-08-21-*` rows (created 2026-06-13-era slugs re-surfaced by backfill), while row 1 (`rmb-desktop-release-0-2-8-dev-5`) has no date prefix at all. Ordering claim **verified TRUE** for the visible window.
- Spot samples (line # in list | uri | created=updated iso):
  - `#1 rmb://events/rmb-desktop-release-0-2-8-dev-5 — 2026-08-22T14:26:30`
  - `#20 rmb://events/2026-08-21-bc-cat-sso-config — 2026-08-21T22:05:49`
  - `#40 rmb://events/2026-08-18-orphan-rebuild — 2026-08-21T20:48:22` ← slug says 08-18, memory written 08-21 (backfill)
  - `#60 rmb://events/2026-08-06-blockcrush-stakeholder-review-migration — 2026-08-21T17:51:20` ← slug 08-06, written 08-21
  - `#80 rmb://events/2026-08-20-gogoblast-added — 2026-08-20T22:06:31`
  - `#100 rmb://events/2026-08-20-comment-cleanup — 2026-08-20T15:01:53`
  - `#120 rmb://events/2026-08-19-fix-deploy-k8s-copyartifacts — 2026-08-19T22:19:22`
  - `#140 rmb://events/2026-08-19-mx-blockcrush-user-dedup — 2026-08-19T17:30:24`
  - `#160 rmb://events/2026-08-19-draft-aliyun-procurement-ticket-v3 — 2026-08-19T17:06:44`
  - `#180 rmb://events/2026-08-18-valkey-test-deploy — 2026-08-18T23:08:11`
  - `#200 rmb://events/2026-08-18-starlink-bbc-gunicorn-chart — 2026-08-18T18:52:41`

### 2. The 200 cap — how much is hidden?

- `ls rmb://events/` returns **exactly 200** rows; visible window spans only **2026-08-18T18:52 → 2026-08-22T14:26 (~3.8 days)**.
- Query 1: `time ~/.rmb/bin/rmb search "deploy build server work event" --scope=memory --k=1000` (real **0.174s**) → **639 unique** `rmb://events/*` uris.
- Query 2: `~/.rmb/bin/rmb search "fix incident migration release pipeline starlink jenkins k8s helm airflow dag" --scope=memory --k=1000` (real 0.182s) → **688 unique** event uris.
- **Union: 1,327 distinct event uris** ≥ the 200 visible → **at least ~85% of all events are unreachable via `ls`**. (True total is higher still: two semantic queries cannot recall everything.) Month histogram of the union's dated slugs: `2026-03: 2, 2026-04: 4, 2026-05: 110, 2026-06: 437, 2026-07: 327, 2026-08: 264` + **183 undated** slugs.
- Pagination attempts all fail:
  - `~/.rmb/bin/rmb ls rmb://events/2026-06-30` → **`ls: inspect/ls: no such column: uri`** (raw internal SQL error leaking through; returned in 0.014s)
  - `~/.rmb/bin/rmb ls rmb://events/2026-06` → same error (the "1 line" result is the error text itself).
  - Daemon HTTP: `/healthz` → `{"status":"ok","driver":"mattn/go-sqlite3+cgo-vec","sqlite":"3.53.4"}`; `/` redirects to `/ui/` (RMB Desktop SPA). Routes found in the UI JS bundle: only `api/v1/onboarding`, `api/v1/config/test/llm`, `api/v1/config/test/embed`, `/healthz`. `/ls?uri=...`, `/api`, `/health` → 404. **No list/count/pagination endpoint exists.**
  - `ls rmb://scenes/` → exactly **200**; `ls rmb://sessions/` → exactly **200** (0.013s). All containers share the same cap.
- Scenes total: `search "session summary conversation" --scope=scene --k=1000` (0.152s) → **1,000 unique scene uris = exactly k**, i.e. scenes ≥ 1,000 (k-saturated), vs 200 visible.

### 3. Date-less slugs (`sqlite-choice`, `rmbd-sqlite-architecture`, `duckdb-to-postgres-pbp`)

All three were **backfilled in one batch on 2026-08-21 from a single scene**:

- `rmb://events/sqlite-choice` — created=updated **2026-08-21T21:12:00.449**, v1, scenes: `cdccc2a4-f2c5-5fa3-bd3e-a111ef85c229`
- `rmb://events/rmbd-sqlite-architecture` — **2026-08-21T21:12:00.448**, v1, same scene
- `rmb://events/duckdb-to-postgres-pbp` — **2026-08-21T21:12:00.445**, v1, same scene
- Source scene `rmb://scenes/cdccc2a4-...` — created **2026-08-21T21:09:55**, `display_name: "decisions"`, `session_id: 589b74b6-...`. Its body contains the real dates in prose: "On 2026-07-11 PBP migrated from DuckDB/OSS Parquet to Postgres (pbp_db)".

Consequences:
- **When did they happen?** Not answerable from the event itself — `created_at` records when the memory row was written (backfill), not when the decision occurred. The true date lives only inside free-text bodies (`cat`), e.g. 2026-07-11 for the PBP migration.
- **Discoverability-by-time is broken for them**: they sort into 2026-08-21 (backfill day) in the timeline, and any slug/date-prefix mental model (every other event is `YYYY-MM-DD-*`) misses them. This is systemic: **183 of 1,327 (~14%) searchable events have no date prefix** (e.g. `agent-driven-ci-deploy-shipped`, `knn-vector-recall-deploy`, `t2-backfill-121-sessions`, `restart-jenkins-systemctl`).
- **Duplicate from the backfill**: `rmb://events/2026-07-11-pbp-to-postgres` (created 2026-08-21T21:28:38, same source scene `cdccc2a4`) states the identical fact as `duckdb-to-postgres-pbp`. One decision → two events, created 16 minutes apart in the same batch.

### 4. "What did I do most recently?" — search vs ls

`~/.rmb/bin/rmb search "recent work" --k=10` (real **0.164s**), all 10 hits dated via `meta`:

| # | Hit | Type | Date |
|---|-----|------|------|
| 1 | `rmb://events/2026-06-02-may-work-summary` | event | **2026-06-02 (~3 months old)** |
| 2 | `rmb://scenes/7a5a328c-...` | scene | 2026-08-12 |
| 3 | `rmb://skills/draft-aliyun-procurement-ticket` | skill | undated |
| 4 | `rmb://entities/hyx` | entity | undated |
| 5 | `rmb://scenes/a53dc775-...` | scene | 2026-08-12 |
| 6 | `rmb://skills/herdr` | skill | undated |
| 7 | `rmb://entities/sha-mingkun` | entity | undated |
| 8 | `rmb://scenes/27a11219-...` | scene | 2026-08-12 |
| 9 | `rmb://skills/jump-hs99-vip` | skill | undated |
| 10 | `rmb://entities/yu-xiangli` | entity | undated |

- **Hits from the last 7 days (since 2026-08-15): 0 / 10.** Newest content surfaced: 2026-08-12 (10 days stale).
- Meanwhile `ls rmb://events/ | head` shows 15+ events created 2026-08-18→08-22 (openresty cutover, bbc-1573 nebula prod deploy, starlink STP hard-delete, rmb-desktop 0.2.8-dev-5…). **Search surfaces none of the newest events** — ranking is purely semantic (the literal word "recent" matched a May-work-summary event and old profile-ish scenes/entities). The agent guide explicitly warns about this, but that means the answer to "what did I do recently?" requires knowing to use `ls`, whose cap (Finding 2) then limits the answer to 4 days.

### 5. Immutability — created_at vs updated_at

- Full sweep of all 200 visible events: **200/200 have `created_at == updated_at`** and **version histogram: {1: 200}**. No evidence of any event ever being updated in the visible window. `rmb meta rmb://events/sqlite-choice` etc. also show v1, equal timestamps.
- Caveat: immutability is **unverifiable beyond the 200-cap** (no way to enumerate older events); if a worker ever rewrote an event, the only observable effect would be `version > 1` / `updated_at > created_at` — both absent here. Note the ls ordering is by `updated_at`, so an edited old event would resurface at the top of the timeline masquerading as new (rows 15–17, the 2026-06-13 slugs sitting between 2026-08-21 rows, are consistent with old-content memories written/re-anchored on 08-21).

### 6. Scenes ↔ events correlation & the session ladder

- Newest scene `rmb://scenes/850a7f9a-...` created 2026-08-22T14:27:13 (`session_id e5d0e743-...`, display_name "Apps") vs newest event created 2026-08-22T14:26:30 — same session, event written ~43 s before the scene. Where links exist, **scene/event dates correlate tightly** (backfill batch: scene 21:09:55 → events 21:12:00).
- But provenance is thin: **75 / 200 visible events (37.5%) have empty `source_scene_uris`** (`grep -c NONE /tmp/event_scenes.tsv` = 75). The other 125 link to just **71 unique scenes**; of those, **50 are inside the visible 200-scene list and 21 are older than the scenes cap**. Conversely **150 / 200 visible scenes are linked by no visible event**.
- Session ladder is broken: `~/.rmb/bin/rmb ls rmb://sessions/589b74b6-0b5b-478a-a0fe-bab8a0887b0a/` and `.../e5d0e743-594b-4c22-9d4a-da96ce998e8c/` (the **current day's** session!) both fail with **`ls: inspect/ls: load session: sql: no rows in result set`**, and neither id appears in the 200-row `ls rmb://sessions/` list. So events → scene → session → turns cannot be walked for sampled sessions (either sessions are aggressively purged/compacted, or the sessions path is buggy).

---

## Findings

1. **Ordering claim is TRUE within the visible window** — 0 inversions across all 200 metas; ordering is by `updated_at`, which correctly interleaves backfilled old-date slugs (e.g. `2026-06-13-*` rows created during an 08-21 backfill) and date-less slugs.
2. **The 200-item cap hides ≥85% of history with no workaround.** ≥1,327 events exist (union of just two search queries; true total higher); `ls` shows 200 covering only ~3.8 days. Date-prefix scoping (`ls rmb://events/2026-06`) crashes with a leaked SQL error (`no such column: uri`); the daemon HTTP API exposes no list/pagination endpoint (only onboarding/config-test/healthz per the UI bundle). Scenes (≥1,000 vs 200) and sessions (capped at 200) have the same blind spot.
3. **Recency queries via search completely fail**: `search "recent work"` top-10 contains 0 items from the last 7 days; the #1 hit is ~3 months old. Semantic-only ranking + no time boosting makes "what did I do most recently?" unanswerable unless the agent already knows to `ls` — and `ls` only reaches 4 days back (see #2).
4. **Date-less slugs are systemic (183/1,327 ≈ 14%) and time-undiscoverable**: their sort position reflects backfill time (2026-08-21), not decision time (e.g. the PBP migration actually happened 2026-07-11 per the scene body). The named trio all came from one backfill scene (`cdccc2a4`, "decisions", 2026-08-21T21:09–21:12), which also produced a **duplicate event pair** (`duckdb-to-postgres-pbp` ≡ `2026-07-11-pbp-to-postgres`, same source scene, created 16 min apart).
5. **Immutability holds everywhere observable** (200/200 `created==updated`, all v1), but is unverifiable beyond the cap; and `updated_at`-ordering means any future edit would disguise an old decision as a new one.
6. **Provenance chain is broken in both directions**: 37.5% of visible events have no source scene; per-session `ls` errors out even for today's session id; scenes ≥5× more numerous than the events that reference them.

## Pain points (ranked by severity)

1. **[CRITICAL] Timeline blind spot: `ls` caps at 200 with zero pagination, hiding ≥85% of events (~1,100+ events, everything before 2026-08-18).** Browsing history older than 4 days is impossible via the documented interface; the only escape hatch is semantic search, which requires guessing keywords. The date-prefix `ls` workaround crashes with `no such column: uri` (an unhandled SQL error surfaced raw to the user).
2. **[HIGH] Search ignores time entirely, so recency questions get confidently wrong answers**: "recent work" → #1 hit from 2026-06-02, 0/10 hits newer than 10 days, while a dozen events from the last 5 days exist. No `--recent`, no time-decay, no `--since` filter anywhere in the CLI.
3. **[HIGH] Date-less slugs break discoverability-by-time and indicate sloppy ingestion**: ~14% of events have no date prefix; backfilled events carry backfill timestamps, so their position in the timeline is fiction; the same backfill produced duplicate events for one decision. True event dates survive only as prose inside bodies.
4. **[MEDIUM] Provenance gaps**: 37.5% of visible events have no `source_scene_uris`; `ls rmb://sessions/<id>/` fails (`sql: no rows in result set`) even for the current session's id; 150/200 visible scenes link to no visible event. The documented pyramid (session→turn→atom→scene→memory) cannot be traversed downward in practice.
5. **[LOW] Immutability unverifiable at scale**: perfect within the visible 200 (all v1, `created==updated`), but no mechanism exists to audit the ~1,100 events beyond the cap; `updated_at`-based ordering would silently promote any future edit to "newest".
