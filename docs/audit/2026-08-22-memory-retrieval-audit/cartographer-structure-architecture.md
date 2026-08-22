# rmb Memory Store — Structure & Architecture Audit (cartographer)

Date: 2026-08-22 · Scope: READ-ONLY exploration via `~/.rmb/bin/rmb` CLI, daemon HTTP API on 127.0.0.1:19019, and read-only sqlite inspection. ~8 min timebox.

---

## 1. Method

1. `~/.rmb/bin/rmb --help` (returns profile + agent guide + skills catalog — doubles as in-band documentation, ~instant).
2. `~/.rmb/bin/rmb ls rmb://<ns>/` for every namespace: sessions, turns, atoms, scenes, events, entities, preferences, skills, memories (~30–50 ms each).
3. Found the daemon: `ps aux` → `rmbd-desktop serve -config ~/Library/Application Support/rmb-desktop/config.yaml`; `lsof` → sqlite db at `~/Library/Application Support/rmb-desktop/data/rmb.db` (188 MB), held open RW by daemon. Queried with `sqlite3 -readonly` for TRUE counts (the only reliable source).
4. Probed daemon REST: `curl http://127.0.0.1:19019/api/v1/browse/overview` and `/api/v1/browse/memories?limit=&offset=` — HTTP pagination works even though CLI pagination is broken.
5. Sampled taxonomy: 60 entity slugs, 45 preference slugs, ~30 preference/event bodies, `rmb cat`/`rmb meta` spot checks.
6. Timing: every CLI call (ls/cat/meta/search-free) returned in **10–50 ms** — daemon is local and fast. Occasional `sqlite3 -readonly` calls hit `Error: database is locked (5)` under daemon write load; retry after 1s succeeded. No write was ever issued.

---

## 2. Inventory — visible (CLI `ls`, capped at 200) vs TRUE counts

| Namespace | `rmb ls` visible | TRUE (sqlite / daemon `/api/v1/browse/overview`) | Visibility |
|---|---|---|---|
| sessions | 200 | **882** | 23% |
| turns | 200 | **5,837** | 3.4% |
| atoms | 200 | **14,684** | 1.4% |
| scenes | 200 | **5,034** | 4.0% |
| events | 200 | **3,382** (all live, immutable) | 5.9% |
| entities | 200 | **2,126** live (6,510 rows incl. superseded versions) | 9.4% |
| preferences | 200 | **975** live (1,521 rows incl. versions) | 20.5% |
| profile | (singleton `rmb://profile`) | 1 live (166 historical versions) | — |
| skills | 4 (true) | 4 skills / 45 skill_files | 100% |
| agent | — | **0 rows** in db with category='agent' | doc served from elsewhere (binary/config, not memories table) |
| `rmb://memories/` | error | — | `ls: inspect/ls: invalid rmb uri: unknown scope "memories"` |

- daemon overview confirms: `{"sessions":882,"turns":5837,"atoms":14684,"scenes":5034,"memories":6484,"skills":4,"corrections":2}` and memory_by_category `{profile_version:166, events:3382, preferences:975, entities:2126}`.
- **CLI pagination is broken**: `rmb ls rmb://events/ --limit 5`, `--offset 200`, and `?offset=200` all fail with `ls: inspect/ls: no such column: uri` (a bug — flag path queries a `uri` column that doesn't exist in the listed table). The 200-cap is therefore a hard wall from the CLI; only the HTTP API (`?limit=&offset=`, returns `items,total,limit,offset`) or sqlite can enumerate fully.
- `ls` ordering: `updated_at DESC` per the agent guide — newest first, so the cap hides the *oldest* ~90%+ of every tier.

---

## 3. Architecture (inferred)

**Process model**: local Go daemon (`rmbd-desktop`, Mach-O arm64, ~17 MB) launched by the `RMB Desktop.app` macOS app, listening on **127.0.0.1:19019** (HTTP + `/ui/` web app + `/api/v1/...` REST). CLI `~/.rmb/bin/rmb` is a thin client (14 MB binary) speaking to the daemon. Companions: `rmb-app`, `rmb-hook-dual` (bash hook shim), local skill cache at `~/.rmb/skills/`, a `release-keys/` dir, and a `backup-2026-08-16-removed-version/` backup folder.

**Storage**: single sqlite db `~/Library/Application Support/rmb-desktop/data/rmb.db` (**188 MB**, RW-locked by daemon, WAL). Tables: `sessions`, `session_turns`, `atoms`, `scenes`, `memories`, `skills`, `skill_files`, `corrections`, `recall_stats`, `pipeline_state`, `goose_db_version` (goose migrations), plus FTS5 shadow tables (`memories_fts`, `atoms_fts`, `scenes_fts`, `skills_fts`). Embeddings are 4,096-byte blobs stored inline in `memories`/`atoms`/`scenes` rows (→ 1024-dim float32, or 512-dim float64), i.e. hybrid vector + FTS search.

**T0–T3 pyramid — all tiers ARE reachable from the CLI** (guide undersells this):

| Tier | URI | Table | `rmb ls` | `rmb cat` |
|---|---|---|---|---|
| T0 sessions | `rmb://sessions/<uuid>/` | sessions | ✅ (lists child turns+atoms) | ✅ |
| T1 turns | `rmb://turns/<uuid>` | session_turns (`messages_json`) | ✅ | ✅ raw JSON of user+assistant exchange |
| T2 atoms | `rmb://atoms/<uuid>` | atoms (category, priority, content) | ✅ | ✅ single extracted fact |
| T2 scenes | `rmb://scenes/<uuid>` | scenes (per-session summary) | ✅ | ✅ summary text |
| T3 memories | `profile / entities/ preferences/ events/` | memories (versioned, `superseded_at`) | ✅ (except `memories/`) | ✅ |

- `session_turns.l1_status` (pending/…) + `pipeline_state` (882 rows = 1/session) ⇒ per-session async distillation pipeline (hook-submit → ingest → L1 atom extraction → scenes → T3 merge), with `/api/v1/debug/pipeline/*` endpoints for stuck/dry-run/requeue.
- **Versioning**: memories are append-only rows; new version gets new `id`, old gets `superseded_at` set. Events are immutable/never superseded (3,382 = 3,382 live). Entities churn the most: **max version 131**, avg 3.06 across live entities (`rmb://entities/blockcrush` is at v89 with 113 source scenes cited).
- `corrections` table (3 rows) stores user-authored fact overrides in natural language (zh + en), e.g. `我是李广慧，郭佳锋不是我，他是星链 Starlink 的产品经理` targeted at `rmb://profile`.
- `recall_stats` (1,089 rows) counts per-URI search hits — feedback loop for ranking.

---

## 4. Taxonomy (sampled 60 entities, 45 preferences, ~30 bodies)

**Entities — themes** (live 2,126):
- **Projects/games** (dominant): blockcrush, bbc, blockblast, block-bloom, nebula, jdmj/mjwd, mypast, rmb, rmb-desktop, rmbd, pbp, pgpour, tutu, starmap…
- **Infra/hosts/tools**: jenkins, openresty, kong, ack-starlink-dev, archteam-* (67 hosts!), aliyun-*, litellm (12 variants), airflow, yunxiao, dockerauth-vpc…
- **People**: only a handful — `li-guanghui`, `xiaokun` (小捆 — actually "a system that creates solutions", mis-slugged as person-like), `zhang-*` (12, e.g. zhang-san prefixes).
- **Massive ecosystem fragmentation**: prefix clusters among live entities: `starli*` **257**, `staror*` 77, `archte*` 67, `aliyun*` 30, `blockc*` 28, `blockb*` 25, `prd-bl*` 10, `nebula*` 10. The Starlink platform is shattered into hundreds of per-topic entities.

**Preferences — themes** (live 975): genuinely behavioral for the most part — language (report-language, communication-language, doc-language, docs-language), tooling (python-stdlib-only, sqlite, postgres, docker-registry-mirrors, package-mirrors), safety (dry-run-before-prod, preview-before-delete, review-not-modify, secrets-in-git, public-exposure-auth), ops conventions (naming-consistency, upstream-naming, prod-ecs-type, redis-per-env, dedicated-redis-per-env, ecs-contract-family-check, procurement-package-format, oa-form-wording).

**Slug conventions**: kebab-case is near-universal ✅. Events are *supposed* to be `YYYY-MM-DD-<slug>` (2,945 comply). **Language mix**: bodies are overwhelmingly English even though the user is zh-primary — Chinese abstracts: entities 14%, events 4%, preferences 2%. The profile itself is written in Chinese. Corrections arrive in both languages.

---

## 5. Structural smells

1. **437 events without date prefixes** (13% of 3,382), e.g. `rmb://events/purchase-nikon-z30`, `install-nx-studio`, `reorganize-zsh-config`, `blockcrush-minio-oss-migration`, `unset-commit-e1aa0d6` — these look like pre-convention or backfilled rows and are invisible to date-browsing.
2. **Backfill flood**: events created per day — 2026-08-13: **1,666**, 08-12: **572** (≈66% of all events in 2 days), vs. steady-state 37–102/day. These are *retro-dated* events (`2026-06-04-agentrun-mandarin-report`, `2026-06-08-aios-…`) — a bulk migration from a prior system (mypast/mem9) imported with old dates but new `created_at`.
3. **Category leakage / misfiling**:
   - Preferences that are facts, not AI-behavior: `rmb://preferences/prod-ecs-type` ("production server is an Aliyun ECS ecs.g9i.xlarge" — an entity/fact), `rmb://preferences/aliyun` ("User uses Aliyun with the caikuai-cn profile").
   - Entities that are events/actions: `work-item-770`, `sync-yunxiao-info-back-to-starlink-db`, `flush-one-tag-solution-yunxiao-info-dag`, `build-34-dataanalysis`.
   - `xiaokun` is a *system* but slugged like a person; people vs systems not distinguished.
4. **Cross-category slug collisions (29)**: same slug live in two categories, e.g. `aliyun` as both `rmb://entities/aliyun` and `rmb://preferences/aliyun`; also `cursor`, `deepseek-v4-flash`, `ansible`, `atlas`, `cloudflare`… Ambiguity for `cat`/search disambiguation.
5. **Near-duplicate topics**: `doc-language` vs `docs-language` (both live, near-identical body); `report-language` vs `communication-language` overlap ("reports in Mandarin" stated in both); `redis-per-env` vs `dedicated-redis-per-env`; `package-mirrors` vs `mirror-consolidation` vs `aliyun-mirrors` vs `aliyun-pypi-mirror`.
6. **Very large memories**: top bodies — `rmb://entities/rmb` 12,310 chars, `blockcrush` 8,471, `starorigin` 7,235, `mypast` 6,254, `jenkins` 6,100, `rmb-desktop` 5,764, `bbc` 5,735… blockcrush's body is a 100+ line runbook-grade document mixing architecture, credentials locations, schema, timezone, CDC — closer to a skill/runbook than an entity memory. Bloated bodies dilute embeddings and bloat context on recall.
7. **Version churn / dead weight**: 5,095 of 11,579 memory rows (44%) are superseded versions kept in-table; profile has 166 versions. Some supersede chains flicker (aliyun v9→v10 within 7 seconds on 08-17 — pipeline re-distilled twice in a row).
8. **`rmb://memories/` not listable** and `rmb://agent` doc not in the memories table (0 rows) — the "agent guide" is injected from elsewhere, so the memory system's own config is outside its own store.
9. **CLI pagination bug** (see §2) makes the 200-cap a hard ceiling for agents that only use the CLI — an auditing agent literally cannot see 98.6% of atoms via `ls`.

---

## 6. Pain points, ranked by severity

| # | Severity | Pain point | Evidence |
|---|---|---|---|
| 1 | **High** | CLI `ls` hard-capped at 200 with **broken pagination flags** (`--limit/--offset` → `no such column: uri`); agents can't enumerate >9% of entities, >3% of scenes/turns. Only HTTP API or sqlite reveal truth. | §2 |
| 2 | **High** | Starlink ecosystem **fragmented into 257 `starli*` + 77 `staror*` + 67 `archte*` entities**; related facts scattered, recall must hit many slugs; near-dup topics (`doc-language`/`docs-language`) never merged. | §5.5, §4 |
| 3 | **High** | **Giant memory bodies** (up to 12K chars) blur entity vs runbook/skill; huge `source_scene_uris` arrays (113 scenes) inflate rows; embedding dilution likely. | §5.6 |
| 4 | **Medium** | **13% of events lack date prefixes** (437), breaking the dated-event convention and timeline browsing; also events-that-are-entities and preferences-that-are-facts misfiling. | §5.1, §5.3 |
| 5 | **Medium** | **Backfill flood** (2,238 events on 08-12/13, retro-dated to June) dominates the store; steady-state signal vs migrated noise is indistinguishable via `created_at`. | §5.2 |
| 6 | **Medium** | **Version churn**: avg 3.06 versions/entity, max 131; double-distill within seconds; 44% of rows are dead superseded versions inflating the 188 MB db. | §5.7 |
| 7 | **Low** | Cross-category slug collisions (29, e.g. `aliyun` entity+preference) create URI ambiguity. | §5.4 |
| 8 | **Low** | Language inconsistency: zh user, ~96% en memories, zh profile, mixed zh/en corrections — no policy encoded. | §4, §5 |
| 9 | **Low** | `rmb://memories/` unlistable; `agent` doc lives outside the store; sqlite reads can hit `database is locked` under daemon write bursts (retry needed). | §2, §1 |

**Bottom line**: a clean, fast (10–50 ms), well-layered local daemon+sqlite architecture (T0→T3 all CLI-reachable, hybrid FTS+vector, versioned memories, corrections loop), whose main structural risks are (a) a hard 200-row CLI visibility cap with broken pagination, (b) entity fragmentation around the Starlink ecosystem, and (c) convention drift in slugs/dates/categories amplified by a one-time bulk backfill.
