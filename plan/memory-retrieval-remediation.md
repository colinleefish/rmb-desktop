# Memory Retrieval Remediation Plan

**Date**: 2026-08-22
**Problem statement**: `docs/audit/2026-08-22-memory-retrieval-audit/MASTER-REPORT.md` (4 sub-agent reports alongside)
**Status**: proposed

Grounded in code as of `bce41e1` (0.2.8-dev.5). Every workstream names the files involved and an acceptance test.

---

## Root causes found in code

| # | Symptom (audit ref) | Root cause in code |
|---|---|---|
| RC1 | `ls` capped at 200, date-suffix `ls` crashes (`no such column: uri`) | `internal/inspect/inspect.go` lsScope default branch: every container query is `... LIMIT 200` hardcoded; suffix path does `q += " AND uri = ?"` **after** `LIMIT 200` → broken SQL. No `--limit/--offset/--since` flags exist anywhere in `cmd/rmb`. (The webui's `internal/browse/browse.go` already paginates properly — the CLI just never got it.) |
| RC2 | Search ignores time; "recent work" → 3-month-old #1 | `internal/recall/search.go`: RRF fusion of vector(0.7)+FTS(0.3) per tier, merged across tiers. No timestamp enters scoring; no `--since` filter; scores not printed. |
| RC3 | Same fact floods results from 3 tiers | Final merge in `search.go` dedupes only by exact URI — scenes/events/preferences carrying identical text are distinct URIs, so all survive. |
| RC4 | Duplicate memories never merge (`doc-language`×3, `redis-per-env`×2, `blockcrush`/`block-crush`) | `internal/worker/memory/worker.go`: L3 buckets atoms by `(category, slug)` and slugs are LLM-chosen at extract time. Divergent slug → parallel bucket → parallel version chain forever. No embedding-similarity check before `insertMemory`. |
| RC5 | Backfill events carry write-time timestamps; ~14% slugs lack dates | `memories` has no `occurred_at`; events dated only by slug convention; `created_at` = write time. |
| RC6 | `ls rmb://sessions/<id>/` fails even for today's session | `inspect.go` lsSession/catSession look up by `session_key`, but scene meta exposes `session_id` — the pyramid can't be walked. |
| RC7 | Skills pollute results (`draft-aliyun-procurement-ticket` in ~40% of top-5s) | Skill tier fused with equal weight into the same merge; `recall_stats` (search_count vs activation) is collected but unused for ranking. |
| RC8 | Events record what, never why/outcome | L2/L3 extraction prompts ask for actions/facts only; no rationale/outcome/refs fields in the distill template. |

---

## Workstreams

### WS1 — CLI reachability (fixes RC1) · *Phase 1, ~2-3 days*

1. Fix the SQL construction bug in `lsScope` (build `WHERE ... AND uri = ?` before `ORDER/LIMIT`).
2. Add flags to `cmd/rmb` `ls`: `--limit=N` (default 200), `--offset=N`, `--since=<date|Nd>` (server-side `WHERE updated_at >=`), `--count` (print total + window). Thread through `internal/httpserver` inspect route → `internal/inspect`.
3. Apply the same to scenes/atoms/turns/sessions branches (they share the hardcoded 200).
4. Update the embedded agent guide (`internal/agentmemory`) to document pagination + `--since`.

**Accept**: `rmb ls rmb://events/ --offset 200 --limit 100` returns 08-17-and-older events; `rmb ls rmb://events/ --since=7d` ≤ 7 days; `--count` reports 3382.

### WS2 — Time-aware search (fixes RC2, RC7) · *Phase 1, ~3-4 days*

1. `--since` filter on `search` (same predicate as WS1).
2. Recency boost in `recall.Service.Search`: after fusion, multiply score by `1/(1+age_days)^γ` (γ≈0.1–0.2, tunable; suppressible via `--no-recency`). Rationale-preserving: pure-semantic behavior stays available.
3. Print fused score per result (sub-agents asked for confidence signal).
4. Skill-tier damping in default scope: cap skills at 1 slot unless `--scope=skill`; long-term, demote skills whose `recall_stats` shows high search-impressions but zero activations.
5. Small popularity prior from `recall_stats.search_count` (log-scaled) — feedback loop the schema already supports.

**Accept**: `search "recent work"` top-10 ≥ 8 items from last 7 days; eval harness (WS8) shows no recall@5 regression on the audit's 8 questions.

### WS3 — Cross-tier result dedup (fixes RC3) · *Phase 1, ~2 days*

In the final merge step of `search.go`: fetch embeddings for candidate hits (already stored), collapse hits whose cosine > 0.98 (v1: exact-normalized-abstract hash match), keep highest-ranked, annotate `(dup of rmb://scenes/… ×2)`.

**Accept**: "openresty dynamic dns" returns the 5 distinct facts once each instead of 5/6 slots being the same fact.

### WS4 — Ingest dedup & one-time reconciliation (fixes RC4) · *Phase 2, ~1-2 weeks*

1. **Slug canonicalization at bucketing**: before creating a bucket, fuzzy-match the proposed slug against active slugs in the category (normalized edit distance + embedding of slug+abstract; threshold tuned on the known clusters). Reuse the existing slug → existing version chain finally absorbs the fact.
2. **Pre-insert similarity check**: in/around `persistMemory`, cosine the distilled body embedding vs active memories in category; > 0.95 → supersede-and-merge instead of parallel insert.
3. **One-time reconciliation (goose migration + LLM-assisted, backup first)**:
   - Merge the 4 confirmed clusters: `redis-per-env`≡`dedicated-redis-per-env`; `release-auto-upload`≡`release-authorization`; `documentation-language`/`doc-language`/`docs-language` (resolve the zh-only vs bilingual contradiction — ask the user once); `blockcrush`/`block-crush` (resolve DWB db name + gql tag contradictions from current infra, not memory).
   - Resolve 29 cross-category slug collisions.
   - Re-slug 437 date-less events (extract date from body → `YYYY-MM-DD-` prefix; see WS6).
4. **Entity fragmentation** (257 `starli*` etc.): do NOT auto-merge infra entities. Build a review flow (webui page or `rmb doctor --merge-suggestions`) listing cosine>0.9 clusters; user approves; merges via the existing supersede machinery.

**Accept**: re-ingesting a session containing the redis fact bumps the existing `redis-per-env` version, not a new slug; cluster count report from `rmb doctor` trends to 0.

### WS5 — Distill *why* and *outcome* (fixes RC8) · *Phase 2, ~1 week, prompt+schema work*

1. L2 extraction prompt: emit atoms for decision rationale, rejected alternatives, verification/outcome — not just actions.
2. L3 event template: `## Event` with Date / Decision / **Rationale** / **Outcome** / **Refs** (resolver IPs, config keys, Jenkins job names, task-folder paths + 1-line summary of folder contents).
3. Update `internal/agentmemory` guide so recallers know events can answer "why/怎么解决".
4. Promotion bar: single-utterance quirks (cf. `call-user-daddy`) need corroboration before becoming preferences.

**Accept**: replaying the audit's Q1/Q5/Q7 (why rejected / why sqlite / 怎么解决的) against newly distilled equivalents answers rationale+outcome without user round-trip.

### WS6 — Timeline truthfulness (fixes RC5, RC6) · *Phase 3, ~1 week*

1. `occurred_at` column (goose): set from slug date, else LLM-extracted from body, else `created_at`. Backfill rows once.
2. `ls rmb://events/` orders by `occurred_at DESC` by default (`--by=updated` to switch); other containers keep `updated_at`.
3. Enforce `YYYY-MM-DD-` prefix in slug validation at extract time (hard requirement for events).
4. Session ladder: `lsSession`/`catSession` accept both `session_key` and `id`; scene meta additionally exposes `session_key`.
5. Backfill empty `source_scene_uris` (37.5% of visible events) where the linkage is recoverable from atoms.

**Accept**: the 08-21 backfill batch sorts into June/July; `ls rmb://sessions/<scene's session_id>/` lists turns.

### WS7 — Store hygiene & security · *Phase 3, continuous*

1. Superseded-row GC: keep last N versions (N=3?) per URI or age-out > 90 days; today 44% of rows / much of the 188 MB is dead weight.
2. Body size cap at distill (e.g. 4k chars); runbook-grade content (cf. `entities/rmb` 12.3k chars) graduates into the skills system with the entity keeping a pointer.
3. `rmb doctor`: never-recalled ratio (currently entities 75%, prefs 90%), duplicate clusters, date-less slugs, orphan scenes — monthly report.
4. **Secrets**: move LLM/embed keys out of `config.yaml` plaintext (env var / macOS Keychain via the app); rotate the two keys that sat in the config during the audit.

### WS8 — Regression eval harness · *start Phase 1, gates all ranking changes*

Encode the audit as replayable eval (golden file): the 8 realistic questions + typo/cross-lingual probes + "recent work" recency check; metrics: recall@5 of expected URIs, dup-rate in top-5, recency-precision. Every WS2/WS3/WS5 change must pass it; KPIs tracked over time (ever-recalled ratio, median cats per answer).

---

## Sequencing & risk

| Phase | Contents | Unlocks |
|---|---|---|
| 1 (week 1) | WS8 scaffold → WS1, WS2, WS3 | daily-pain P0s gone: full history reachable, recency works, results un-flooded |
| 2 (weeks 2-3) | WS4 (canonicalization + reconciliation), WS5 | new memories stop rotting; why/outcome captured |
| 3 (weeks 4-5) | WS6, WS7, entity merge review flow | timeline truthful, store shrinking instead of growing dark matter |

**Risks**: (a) ranking changes regress precision → gated by WS8, ship behind `--no-recency` escape hatch; (b) auto-merge fabricates "canonical" facts → cross-slug merges require human approval, only exact-cluster reconciliation is scripted; (c) migrations on the 188 MB live db → goose transactional + pre-migration backup (a backup dir already exists from 08-16); (d) prompt changes (WS5) shift atom formats → version the template, A/B on a session replay before rollout.
