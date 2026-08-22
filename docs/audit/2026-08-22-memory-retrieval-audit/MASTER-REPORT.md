# RMB Memory Store — Retrieval Audit (Sub-agent Swarm Test)

**Date**: 2026-08-22 · **Method**: 4 parallel `pi` sub-agents spawned via Herdr in sibling panes (cartographer, treasure-hunter, time-traveler, fact-checker), each given an independent exploratory mission (structure / realistic retrieval / timeline / consistency), plus orchestrator-level analysis of the sqlite store (read-only).

**Sub-reports** (full evidence in each):
- `rmb-audit-cartographer-report.md` — structure & architecture
- `treasure-hunter-report.md` — 8 realistic retrieval questions + 2 adversarial probes
- `REPORT-time-traveler.md` — timeline & recency (meta on all 200 visible events)
- `consistency-quality-report.md` — duplicates / contradictions / cross-refs / language parity

---

## 1. What your memory actually is (architecture)

- **Go daemon** `rmbd-desktop` (127.0.0.1:19019) + thin CLI, sqlite + FTS5 + cgo-vec at `~/Library/Application Support/rmb-desktop/data/rmb.db` (~188 MB), embeddings (1024-dim, bigmodel embedding-3) inline per row. Async L1→L2→L3 distillation pipeline per session.
- **T0–T3 pyramid is fully CLI-reachable** (sessions → turns → atoms/scenes → memories), versioned memories (`superseded_at`), corrections table, per-URI `recall_stats`.

## 2. Key numbers (true counts vs. what the CLI shows)

| Store | True total | CLI `ls` visible | Visibility |
|---|---|---|---|
| sessions | 886 | 200 | 23% |
| turns | 5,841 | 200 | 3.4% |
| atoms | 14,688 | 200 | 1.4% |
| scenes | 5,036 | 200 | 4.0% |
| events (active) | 3,382 | 200 | 5.9% |
| entities (active) | 2,126 | 200 | 9.4% |
| preferences (active) | 975 | 200 | 20.5% |

- Superseded dead rows: 5,095 of 11,579 memory rows (44%). Max entity version: **131** (`aliyun`); profile has **166 historical versions**.
- Backfill flood: **1,666 events created on 2026-08-13 + 572 on 08-12** (≈66% of all events in 2 days), retro-dated to June.
- **Ever-recalled ratio**: entities 24.9%, events 30.2%, **preferences 10.2%** → ~70%+ of the store is dark matter.

## 3. Problems found, ranked

### P0 — The two killers (why retrieval feels harder and harder)

1. **CLI browsing is hard-capped at 200 with broken pagination.** `ls rmb://events/ --limit/--offset` and date-prefix scoping crash with `ls: inspect/ls: no such column: uri`. The visible timeline covers only **~3.8 days** (2026-08-18 → 08-22); ≥85% of events (~1,100+) are unreachable except by guessing search keywords. Scenes/sessions/turns/atoms have the same 200-cap. (The daemon HTTP API `/api/v1/browse/memories?limit=&offset=` paginates fine — the CLI just never exposes it.)
2. **Search ignores time entirely.** `search "recent work"` → #1 hit is from **2026-06-02**, 0/10 hits newer than 10 days, while a dozen events from the last 5 days exist. No `--since`, no time-decay, no recency boost. Recency questions are confidently answered *wrong* unless the agent knows the `ls` workaround — which then only reaches 4 days back (see #1).

### P1 — Structural retrieval pollutants

3. **Same fact lives in 2–3 tiers and floods results.** Default search mixes scenes+memories+skills: for "jenkins deploy job", 8/9 hits were scenes; "openresty dynamic dns" returns the identical fact at 5 of the top 6 ranks (event + scene×2 + preference + event). Scenes add zero info over events in most sampled cases.
4. **No duplicate detection at ingest.** Confirmed clusters: `redis-per-env` ≡ `dedicated-redis-per-env`; `release-auto-upload` ≡ `release-authorization`; a **3-way doc-language split** (`documentation-language`/`doc-language`/`docs-language`) whose bodies *contradict each other* (Chinese-only vs bilingual); mega-duplicates `blockcrush` (v89) vs `block-crush` (v37) with conflicting facts: DWB DB name (`dwb_blockcrush` vs `dwb_blockcrush_client`) and the gql prod tag recorded as "misconfiguration" in one and "expected prod" in the other.
5. **Ecosystem fragmentation.** Starlink is shattered into **257 `starli*` + 77 `staror*` + 67 `archte*` entities**; a "prod servers" question must hit many slugs, and the mega-entities don't even win ranking for their own content.

### P2 — Content quality of what does get retrieved

6. **Events capture *what*, never *why* or *outcome*.** "Why was cluster-admin-toolbox rejected?", "why DuckDB→Postgres?", "怎么解决的?" are all unanswerable — 3 of 8 realistic questions failed on missing rationale/resolution, not on ranking. The one rationale that exists (sqlite choice) is buried in a *scene* and took 5 query reformulations.
7. **One-liner depth.** Events are single sentences; "details" questions (openresty resolver IPs, photo pipeline) dead-end or need reassembly from 3+ cats. Pointers to local task folders are stored without any of the findings.
8. **Timeline fiction from backfill.** ~14% of events lack date prefixes; backfilled rows carry backfill timestamps (created 2026-08-21) while slugs/prose claim June/July — `updated_at`-ordered ls makes old decisions masquerade as new. One backfill scene produced a duplicate event pair 16 minutes apart.
9. **Language-asymmetric ranking.** ZH "BlockCrush 生产服务器" → wrong #1 (the DB entity); EN gets it right. ZH/EN parity is not guaranteed despite bilingual corpus. (Typos, however, are handled perfectly; cross-lingual *retrieval* mostly works.)

### P3 — Smaller irritants

10. Skill pollution: `skills/draft-aliyun-procurement-ticket` ranked top-5 in ~6 of 14 unrelated queries.
11. Provenance gaps: 37.5% of visible events have empty `source_scene_uris`; `ls rmb://sessions/<id>/` fails (`no rows`) even for today's session — the pyramid can't be walked downward.
12. Miscategorization (ticket snapshots and team conventions as entities/preferences), joke preferences promoted to durable memory (`call-user-daddy`), giant 12k-char entity bodies (`entities/rmb` 12,310 chars) diluting embeddings, no similarity scores shown in search output.
13. Ops note: `config.yaml` contains live LLM/embed API keys in plaintext.

## 4. What works well (keep these)

- **Speed**: search 0.15–0.26 s, cat/meta 5–25 ms, even at `--k=1000`.
- **Typo tolerance is perfect** in tests; cross-lingual retrieval succeeded on most probes.
- **Cross-reference integrity 100%** (10/10 sampled links resolve; bogus URIs fail cleanly).
- Versioning, corrections, and `recall_stats` exist — the data needed to fix everything above is already in the store.
- `--scope=memory` produces dramatically cleaner results than default scope (immediate mitigation).

## 5. Recommendations (highest leverage first)

1. **Expose pagination in the CLI** (`ls --limit --offset`, fix the `no such column: uri` crash; the daemon browse API already supports it). Even `--since=7d` alone would kill most P0 pain.
2. **Add recency to search**: time-decay boost or a `--since` filter; never let a 3-month-old hit outrank 12 events from this week for recency-intent queries.
3. **Dedupe at ingest**: similarity-check new memories against active ones in the same category (embeddings already exist); run a one-time reconciliation for the 4 confirmed clusters; enforce one canonical doc-language preference.
4. **Collapse tier duplication in results**: dedupe identical/near-identical bodies across scene/event/preference within one result set (show one, annotate copies); consider demoting scenes by default.
5. **Distill *why* and *outcome***, not just actions: add rationale/outcome fields to event extraction; allow a short details block (resolver IPs, config keys, one-line summaries of linked task folders).
6. **Enforce `YYYY-MM-DD-` slug prefixes** for events; record `occurred_at` separately from `created_at` so backfills don't rewrite the timeline.
7. **Use `recall_stats` as a feedback loop**: demote never-activated skills, GC/archive the 70% never-recalled dark matter and 5k superseded rows.
8. Fix the session ladder (`ls rmb://sessions/<id>/`), and move API keys out of plaintext config.

---

*Audit artifacts: this file + 4 sub-reports in `/tmp/rmb-audit/`. Sub-agent panes left idle in the Herdr tab for inspection (cartographer wK:p2, treasure-hunter wK:p4, time-traveler wK:p3, fact-checker wK:p5) — close them when done reviewing.*
