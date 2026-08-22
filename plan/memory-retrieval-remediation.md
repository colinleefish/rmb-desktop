# Memory Retrieval Remediation Plan — v2 (refined)

**Date**: 2026-08-22 · **Supersedes**: v1 (same day, prior commit — kept in git history)
**Inputs**: `docs/audit/2026-08-22-memory-retrieval-audit/` + addendum; follow-up verification queries against the live db (read-only).
**Status**: proposed

This version is the product of an adversarial self-review of v1: every claim re-checked against the database, every fix stress-tested for failure modes, impact re-ranked by query frequency rather than loudness.

---

## 1. What the review corrected in v1

| # | v1 said | Review found | v2 change |
|---|---|---|---|
| C1 | P0 = 200-cap + time-blind search | Result **flooding** (tier duplication + skill pollution) degrades *every* query; the cap only hurts enumeration questions, time-blindness only recency questions. Impact was ranked by loudness, not frequency. | Re-ranked: precision fixes first (§4, P0). |
| C2 | Recency boost: `score·(1/(1+age))^γ`, γ≈0.1–0.2 | **Mathematically unsound on RRF scores.** RRF scores compress into a narrow band (top ≈ 0.0164, rank-3 single-leg ≈ 0.0111 — a 1.5× gap spanning 2 ranks). γ=0.1 over 60 days multiplies by ~0.66 → flips 2-rank gaps, collapsing ranking into near-pure recency. Only γ≈0.01–0.03 is safe, and even then it only breaks near-ties. | Explicit `--since/--until` filters + agent-guide intent routing ("recent" → add `--since=7d`) becomes the primary fix — deterministic, testable, zero precision risk. Decay demoted to a behind-flag experiment, γ calibrated on eval data (§4 P1). |
| C3 | Cross-tier dedup via cosine>0.98 in merge | Treating a symptom. Scenes duplicate memories **by design** — memories carry `source_scene_uris`. The cleaner fix: scenes shouldn't be primary search results at all; they're drill-down targets. The pyramid already defines this flow; the default scope violates it. | Default scope → `memory` + capped `skill`; scene hits suppressed when deterministically linked (`scene ∈ source_scene_uris` of a hit memory — no cosine guesswork); `--scope=scene` stays for explicit need. |
| C4 | Cap skills at 1 slot | Symptom-level. Root cause: the skill tier competes on **vector** similarity, and generic ops-English skill descriptions ("Aliyun procurement for OA…") embed close to everything in a domain-concentrated corpus. Lexical match is the correct signal for skills (distinctive names). | Skill tier = FTS-only + cap 1 in default scope. |
| C5 | (missing) | Q7's "怎么解决的" failure isn't just missing *outcome* fields — resolutions happen in **later sessions** than the problem, so per-session distillation can never link them. v1 had no linking mechanism. | **Event linking at distill time** (retrieve-then-link): related top-5 events injected into the L3 prompt; resolutions emit `Related:` refs. (§4 P2) |
| C6 | (missing) | Atoms (14,717 rows, **all with embeddings + FTS index already built**) are not a searchable scope. They are the richest per-fact tier and would have answered Q8's "details" dead-end without any re-distillation. | Add `--scope=atom`. Cheap: infrastructure exists. |
| C7 | "70%+ dark matter, never recalled" | `recall_stats` instrumentation began ~2026-08-09 (goose v7); the statement is really "not recalled in a 13-day window". Also: **every memory row was written in the last ~20 days** (backfill imported old dates in slugs/prose only). The complaint "harder and harder" is therefore a **growth-rate** problem (≈50–100 events/day steady-state + 2,238-event backfill), not legacy accumulation. | Archival/forgetting policy promoted from a doctor-report bullet to a designed subsystem (§4 P3); audit addendum corrects the stat. |
| C8 | Fuzzy slug matching at bucketing | String-distance matching mis-merges (`bbc-build` vs `bbc-build-2`) and can't see semantics. Better: **retrieve-then-canonicalize** — at L2 extraction, embed the proposed slug/abstract, fetch top-K candidate existing slugs, show them to the extractor ("reuse slug if same subject"). Thresholds calibrated on the audit's labeled duplicate clusters + random negatives. Hysteresis: once merged, prefer the incumbent slug. | Replaces WS4 fuzzy matching. Also fixes **profile churn** (166 versions ≈ 8/day): extend the existing `bucketUnchanged` gate with a body-semantic diff (unchanged body → no new version). |
| C9 | Contradictions: "resolve from infra" | Left the mechanism implicit. The system already has a **corrections** table (3 rows, underused) purpose-built for evidence-based fact overrides. | Contradiction resolution writes corrections; memories are never hand-edited. |
| C10 | `ls` segment = exact-uri filter (fix crash) | Weak contract. Users and agents naturally type `ls rmb://events/2026-08-13-` expecting date browsing. | Segment = **slug/id prefix filter** (`LIKE 'seg%'`) + `--limit/--offset/--since/--until/--count` flags. Same effort, better contract. |
| C11 | (missing) | No query-level telemetry: `recall_stats` knows per-URI hits but nothing about *queries that found nothing useful* (search→no-cat is a failure signal). Tuning C2/C4 without this data is guesswork. | Search query-log (local table): query, scope, top-k, which hits were catted within 10 min. Feeds eval + threshold calibration. |
| C12 | Audit golden success rate 5/8 | Questions were formulated **after** inspecting the store (selection bias) and by testers who knew retrieval was being tested (expectation bias). Real-world success is likely lower. | WS8 golden set derives questions from raw session turns *before* looking at what was distilled; addendum notes the bias. |

## 2. Re-scored problem model (by share of queries hurt)

1. **Precision decay on every query** — tier flooding + skill pollution + duplicate memories competing for top-k. Gets worse as store grows. *(C1, C3, C4)*
2. **Growth without forgetting** — 11.5k memories in 20 days, 90% untouched in-window; embeddings/FTS candidates grow, near-duplicate density rises, precision decays further. This is the engine of "harder and harder". *(C7)*
3. **Enumeration blindness** — 200-cap, no pagination/prefix browsing. *(C10)*
4. **Recency questions answered wrong** — no time filters. *(C2)*
5. **Follow-up questions unanswerable** — no why/outcome, no cross-session resolution links, details live in unreachable tiers. *(C5, C6)*
6. **Trust erosion** — live contradictions, undiscoverable dates, broken session ladder. *(C8, C9, C10)*

## 3. Root causes (updated; file-level)

| RC | Cause | Where |
|---|---|---|
| RC1 | `LIMIT 200` hardcoded in all `lsScope` branches; segment path appends `AND uri=?` after LIMIT (broken SQL) | `internal/inspect/inspect.go` |
| RC2 | RRF fusion (0.7/0.3) with no time signal, no filters; scores unprinted | `internal/recall/search.go` |
| RC3 | Default scope includes scenes; final merge dedupes exact URIs only; tiers compete as equals | `internal/recall/search.go` |
| RC4 | Skill tier fused on vector+FTS at equal weight | `internal/recall/search.go` |
| RC5 | L3 buckets by LLM-chosen `(category, slug)`; no canonicalization against existing slugs; no pre-insert similarity check; `bucketUnchanged` gates on source-set only → profile churn | `internal/worker/memory/worker.go` |
| RC6 | No `occurred_at`; event dates live only in slug convention/body prose | schema |
| RC7 | `lsSession`/`catSession` resolve `session_key` while scenes expose `session_id` | `internal/inspect/inspect.go` |
| RC8 | Extraction/distill prompts capture actions only; no rationale/outcome/related fields | L2/L3 prompts |
| RC9 | Atoms not exposed as a search scope despite embeddings+FTS existing | `internal/recall/search.go` |
| RC10 | No query telemetry (only per-URI `recall_stats`) | new |
| RC11 | No archival/forgetting subsystem | new |

## 4. Design principles

1. **Search returns distilled truth; depth comes by drill-down.** Memories are primary results; scenes/atoms are reached via provenance links or explicit scopes — not sprayed into every result list.
2. **Explicit time beats implicit decay.** Filters + guide-taught intent routing first; silent ranking changes only after eval data justifies them.
3. **Link, don't guess.** Prefer deterministic relationships (`source_scene_uris`, incumbent slugs, exact evidence) over similarity heuristics; similarity only where no link exists, with calibrated thresholds.
4. **Forgetting is a feature.** A memory system that only grows gets harder to retrieve from by construction. Archival must be reversible and evidence-gated.
5. **The agent guide is the retrieval API.** Every behavior change ships with a guide update in the same release; agents re-read it every session.
6. **Evidence over edit.** Contradictions resolve via the corrections mechanism against live infrastructure, never by hand-editing memories.

## 5. Workstreams

### P0 — Precision & reachability (days 1–3)

**P0.1 Fix `ls` (RC1, C10).** Segment = slug/id prefix (`LIKE 'seg%'`, prefix-anchored, index-friendly); add `--limit` (default 200) / `--offset` / `--since` / `--until` / `--count` (total + window). Thread CLI → httpserver inspect route → inspect service. Apply to all containers.
*Accept*: `rmb ls rmb://events/2026-06` lists June events (currently: SQL error); `--offset 200` returns pre-08-18 rows; `--count` reports 3,382.

**P0.2 Search scope & tier redesign (RC3, RC4, C1, C3, C4).**
- Default scope: `memory` + `skill` (FTS-only, capped at 1 slot). Scenes leave the default.
- Link-based suppression: if an explicit multi-scope search returns a scene that ∈ `source_scene_uris` of another hit, suppress it and annotate the memory `(+scene depth: rmb://scenes/…)` — drills down deterministically.
- `--scope=scene` unchanged for explicit use; `--scope=memory,scene,...` still composable.
- Print fused score per result (confidence signal; agents currently guess).
*Accept*: "openresty dynamic dns" default search: 0 suppressed-duplicate scenes in top-5, ≥4 distinct facts; the aliyun skill appears in ≤1 of 10 unrelated golden queries' top-5 (was ~6/14).

**P0.3 Time filters (RC2, C2).** `--since/--until` on search (v1 semantics: filters on `updated_at`; documented caveat: for immutable events this is write-time until P3.1 lands). No decay in this phase.
*Accept*: `search "work" --since=7d` returns only last-7-day items; golden recency questions answered via guide-taught `--since` workflow.

**P0.4 Agent guide v2 (principle 5).** Teach: recency intent → `--since`; depth → cat `source_scene_uris`; details → `--scope=atom` (P1.1); enumeration → prefix `ls` + pagination; skills outrank defaults. Guide diff reviewed like code.
*Accept*: fresh sub-agent replay of the audit's 8 questions + 2 probes using only the guide ≥ v1 results with fewer cats.

**P0.5 Eval scaffold (C12).** Golden set built from **session turns sampled across the 20-day window, questions written before inspecting distilled memories**; expected answers pinned from turns. Metrics: recall@5, dup-rate in top-5, recency-precision, cats-per-answer. Runs on every PR touching recall/inspect.
*Accept*: harness green on HEAD, CI-wired, v1-baseline numbers recorded.

### P1 — Depth & data (week 1–2)

**P1.1 `--scope=atom` (RC9, C6).** Vector+FTS over atoms (embeddings exist for 14,717/14,717; `atoms_fts` built). Answer-attribution: results annotated with parent scene/session for drill-down.
*Accept*: golden Q8 ("openresty resolver IPs") answered from an atom hit without re-distillation.

**P1.2 Query telemetry & heat counter (RC10, C11, §10).** Local tables: (a) query log — query, scope, k, top-k uris, ts; (b) join search→cat within 10 min (successful retrieval). Add `heat` + `last_use_at` to recall_stats: `heat = heat·e^(−Δt/τ) + w` on qualifying use (τ≈30d; w=1 cat, 0.3 meta, search only if cat follows ≤10min; never bare impressions). Applies to memories and skills. Dashboard: zero-cat search rate, heat concentration (feedback-loop alarm).
*Accept*: heat column live on all recall events; two weeks of data collected; aliyun skill heat ≈ 0 while jump-hs99-vip heat is high.

**P1.3 Usage-heat ranking (D3, §10).** Replaces the v2 recency-decay experiment: `final = rrf + α·log(1+heat) + β·e^(−age/14d)`, α/β bounded to ≤~10% each of median top-1 RRF score, `--no-boost` escape hatch, config-tunable. Rollout: counter ships in P1.2 → accumulate ≥30d → enable behind flag → default-on only after eval shows zero recall@5 regression on golden set. Cold-start novelty term ensures new memories surface before heat accrues.
*Accept*: golden recency questions pass without `--since`; recall@5 on golden set unchanged; heat concentration stable in doctor.

**P1.4 Cosine cross-tier suppression (fallback).** Only for unlinked near-dups (scene ∉ source_scene_uris but cos>0.98): suppress lower tier. Thresholds calibrated on labeled clusters.

### P2 — Stop the rot at ingest (weeks 2–4)

**P2.1 Retrieve-then-canonicalize (RC5, C8).**
- L2 extract prompt receives top-K (≤20) candidate existing slugs per category (embedding retrieval on slug+abstract) with instruction: same subject → reuse slug; genuinely new → propose slug with enforced `YYYY-MM-DD-` prefix for events.
- L3 `persistMemory`: pre-insert similarity check vs same-category actives; cos>τ → merge into incumbent (new version), τ calibrated on the 4 audit clusters + random negatives; hysteresis: incumbent wins ties.
- Profile churn: extend `bucketUnchanged` with body-comparison (exact-normalized v1; semantic v2) — 166 versions → converge.
*Accept*: replay a session containing the redis-per-env fact → bumps existing memory's version, creates no new slug; profile version rate < 1/day on unchanged days.

**P2.2 Event linking + why/outcome (RC8, C5).**
- L3 event prompt: inject top-5 related active events; extract Decision / **Rationale** / **Outcome** / **Related:** (uri refs, incl. "resolves rmb://events/…") / **Refs** (IPs, config keys, job names, task-folder + 1-line contents summary).
- v1 stores links in body template (no migration); `related_uris` column only if query patterns demand it.
- Promotion bar: single-utterance preferences (cf. `call-user-daddy`) require corroboration across sessions.
*Accept*: replay audit Q1/Q5/Q7 → rationale + resolution retrievable without user round-trip; resolution event links its problem event.

**P2.3 One-time reconciliation (C8, C9).** Pre-backup (goose transactional + sqlite file copy).
- Scripted: merge 4 duplicate clusters; re-slug 437 date-less events (date from body → prefix; unknowable → `undated-` prefix + doctor flag); resolve 29 cross-category slug collisions.
- Human-in-loop: blockcrush/block-crush contradictions (DWB db name, gql tag) resolved by checking live infra **via sub-agent + `jump-hs99-vip` skill, read-only** (D4), then recorded as **corrections** (not edits); doc-language 3-way split **resolved per D5**: canonical = `docs-language` (中文 technical + bilingual general), supersede `documentation-language` + `doc-language`, correction entry pinned.
*Accept*: `rmb doctor --duplicates` reports 0 known clusters; corrections table carries the resolved contradictions with evidence refs.

### P3 — Truthfulness, forgetting, hygiene (weeks 4–6)

**P3.1 `occurred_at` (RC6).** Column + backfill (slug date → body-extracted date → created_at fallback); events `ls`/`--since` switch to `occurred_at` (`--by=updated` opt-out); other containers keep `updated_at`.
*Accept*: 08-21 backfill batch sorts into June/July; `--since` on events filters by when things happened.

**P3.2 Session ladder (RC7).** `lsSession`/`catSession` accept `session_key` **and** `id`; scene meta exposes both. Backfill empty `source_scene_uris` where recoverable via atoms (37.5% of visible events).

**P3.3 Archival / forgetting (RC11, C7, D2=90d).** New `archived_at` column. Policy (tunable): heat ≈ 0 AND no qualifying use in **90 days**, not profile/correction-linked, superseded-chain cold → **doctor proposes** archive; bulk-archive on approval; archived rows leave default search, remain `cat`-able and restorable. Never auto-delete. Evidence tiers (turns/atoms/scenes) exempt forever (§9.2 invariant).
*Accept*: doctor's first run proposes a reviewable list (expected small until telemetry matures); archive+restore round-trips; default-search candidate pool measurably shrinks.

**P3.4 Hygiene.** GC superseded versions (keep last 3 or 90 days — 44% of rows today); body cap at distill (4k chars) with runbook-grade content graduating to skills (entity keeps pointer); `rmb doctor`: duplicate scan (cos>0.9 pairs → LLM-diff → proposed corrections), contradiction scan, date-less slug count, orphan scenes, archive candidates, zero-cat queries.

**P3.5 Secrets.** Move LLM/embed keys out of `config.yaml` plaintext → env/Keychain via the desktop app; **rotate both live keys and any in `config.yaml.bak-stargate`**; scrub backups.

## 6. Decision log (need user input)

| D1 | Default scope change (scenes out) | **RESOLVED 2026-08-22 — yes, reframed** (see §9.2): memory-primary default stands, not because scenes are redundant but because evidence must be *reachable by drill-down*, not sprayed into rankings. Safeguards added: version-as-trust-signal, `--scope=atom` for detail queries, evidence tiers exempt from archival. |
| D2 | Archive threshold | **RESOLVED 2026-08-22 — 90 days** (one quarter). Archive candidacy = heat ≈ 0 AND no use in 90d; doctor-proposed, user-approved, reversible; evidence tiers exempt. |
| D3 | Decay default | **RESOLVED 2026-08-22 — usage-heat ranking, ON (phased)**. User wants a time factor plus a popularity factor ("often searched = hot"). Design per §10: cat/meta-weighted exponentially-decayed heat counter (never raw search impressions — rich-get-richer trap), bounded additive bonus + 14d cold-start novelty, default-on after ~30d telemetry accumulation + eval gate, `--no-boost` escape hatch. |
| D4 | Contradiction evidence gathering | **RESOLVED 2026-08-22 — default: sub-agent with the `jump-hs99-vip` skill** runs read-only evidence checks (DWB db name, gql prod tag) at P2.3 execution time; one approval per cluster before any merge. User can swap to self-run without changing the plan. |
| D5 | doc-language resolution | **RESOLVED 2026-08-22 — bilingual**: technical documentation 中文; general documentation **中英双语 with technical detail** (user decision, matches `docs-language`'s content). Canonical = `docs-language`; supersede `documentation-language` + `doc-language`; correction entry pinned so L3 cannot re-split. |

## 7. Risks

- **Scope-change regression** → P0.4 guide ships atomically with P0.2; eval gates; `--scope=memory,scene` escape hatch preserves old behavior exactly.
- **Canonicalizer mis-merges** → calibrated thresholds + hysteresis + doctor duplicate-scan as safety net; mis-merge recovery = supersede-chain rollback (machinery exists).
- **Migrations on live 188 MB db** → goose transactional + file backup first (precedent: `backup-2026-08-16-removed-version/`).
- **Prompt changes shift formats** → version templates; A/B on session replay before rollout.
- **Telemetry privacy** → local-only table; never leaves the machine; documented.

## 8. Explicitly not doing (now)

- Auto-deleting memories (archive only, reversible).
- Auto-merging entity fragments (257 `starli*` shards) without per-cluster approval — doctor proposes, human disposes.
- Full-text search over raw turns (5,841 transcripts; noisy + large; atoms cover distilled detail) — revisit if telemetry shows demand.
- Semantic (vector) ranking for the skill tier (lexical only, per C4).

---

## 9. Amendment v2.1 — consolidation erosion (external evidence) & D1 resolution

**Input**: Zhang et al., *"Useful Memories Become Faulty When Continuously Updated by LLMs"* (arXiv:2605.12978, May 2026) — provided by the user for D1. Verified against rmb's code (`internal/worker/memory/worker.go`) before adopting anything.

### 9.1 What the paper establishes

1. Continuously LLM-rewritten consolidated memory **erodes**: utility rises, then degrades below the no-memory baseline; in the cleanest case 100%-solved problems fall to 52.6% after 10 rounds of streaming consolidation, while one-shot (Static) consolidation of the same pool stays at ceiling.
2. The failure is the **consolidation loop itself**: each rewrite compounds small abstraction errors; update-after-every-interaction designs are contradicted.
3. **Episodic-only stores are competitive** with consolidated memory across benchmarks; raw episodes are the robust default; abstraction should be *selective, delayed, grounded in recoverable trajectories*.
4. Three mechanisms: **misgrouping** before abstraction (forced early consolidation breaks segmentation that the model gets right given time); **interference** (overgeneralization strips applicability conditions; cumulative distillation accumulates overgeneralized entries at ~5× and garbage at ~20× the rate of per-task distillation); **overfit** to narrow/duplicate streams.
5. Prescription: keep **episodic and schema-forming stores architecturally distinct**; gate consolidation rather than firing it every step.

### 9.2 D1 resolution — memory-primary default stands, with the roles reframed

The paper inverts v2's implicit assumption ("distilled memories are the good tier, scenes are redundant copies"). Scenes/atoms are not noise — they are the **evidence tier**, and the audit already caught the paper's predicted failures in production: rationale surviving only in a scene (sqlite decision), details lost at the memory tier (Q8), contradictions between coexisting memory versions (blockcrush), paraphrase-accretion inside a merged body (doc-language's 5 bullets).

Decision: **yes to D1** — default scope = `memory` + `skill` (FTS, cap 1) — but the justification changes: memories are the *index*; evidence stays *reachable by deterministic drill-down* (`source_scene_uris`, `--scope=atom`). The paper's requirement is recoverable evidence, and rmb satisfies it architecturally — provided we make that an **invariant**: evidence tiers (turns/atoms/scenes) are never GC'd or archived (§9.3-e). Required safeguards shipped with P0.2/P0.4:
- search output carries **version count as a trust signal** (high-churn = heavily rewritten = verify via linked scene);
- guide v2 teaches tier routing: decisions/why → events (immutable) + linked scenes; details → `--scope=atom`; stable facts → entities/preferences.

### 9.3 Code-grounded findings (rmb ↔ paper mapping)

Verified in `worker.go` before amending:

| Paper mechanism | rmb code reality | Fix |
|---|---|---|
| Update-after-every-interaction | `rollup` fires per session batch; only gate is `bucketUnchanged` = source-scene-set equality → profile re-distilled ~8×/day (166 versions) | **(a) Materiality gate**: require N new atoms or semantic delta, not just a new source scene |
| Rewrite-of-rewrite compounding | multi-chunk path distills partials, then re-distills *the distilled partials* (`serializePartialsForLLM`) | **(b) Reduce pass must see atom-level evidence** (or atom excerpts alongside partials), never distilled-only input |
| Near-duplicate stream overfit | L1 re-extracts paraphrased atoms per session; nothing dedupes atoms; merge keeps all variants (doc-language 5 bullets) | **(c) Atom-level near-dup suppression at L1/L2 ingest** (embedding sim vs same-slug neighborhood) |
| Misgrouping / forced early abstraction | slugs are LLM-chosen at extract time; new subjects get rewritten-tier memories immediately (doc-language split; `call-user-daddy`) | **(d) Graduation threshold**: new subjects accrete as atoms/events (append-only, immutable) and graduate to entities/preferences only after K distinct sessions corroborate |
| Stream ≪ Static schedules | L3 is per-subject Static-All over the full atom pool — already the paper's better regime, but erosion still accumulates via (a)–(c) | **(e) Re-consolidation from evidence as maintenance**: `rmb doctor --reconsolidate <uri>` re-distills full evidence in one static pass, replacing eroded chains; also **never delete/GC evidence tiers** (archival in P3.3 applies to the memories table only); doctor flags high-version memories (aliyun v131) for re-consolidation |

### 9.4 Plan deltas

- **P2.1 amended**: consolidation gate (9.3a) and atom dedup (9.3c) join slug canonicalization; body-diff gate generalizes from profile-only to all rewritten categories.
- **P2.2 unchanged** (why/outcome/events) — the paper independently supports append-only event records as the safe tier.
- **P2.3 amended**: prefer `--reconsolidate` (evidence-grounded static re-distillation) over hand-merged cluster bodies where evidence exists; corrections still override facts.
- **P0.2 amended**: version count in search output (trust signal).
- **P0.4 amended**: tier-routing guidance per §9.2.
- **P3.3 amended**: archival scope explicitly excludes turns/atoms/scenes (invariant: evidence is never discarded).
- **New acceptance test**: after gate (9.3a), profile version rate on unchanged-identity days = 0; after (9.3d), no new single-session preference promotions.

---

## 10. Usage-heat ranking design (D3 resolution)

User decision: ranking should carry a time factor **and** a popularity factor ("often-retrieved memories are hot").

**Principle — usage beats exposure.** `search_count` is an impression, not a choice; boosting on it creates a rich-get-richer loop (the aliyun-skill pollution would become permanent). Only qualifying *use* generates heat: cat (w=1), meta (w=0.3), search→cat within 10 min (counts as cat). Bare search hits never count.

**Heat counter**: `heat = heat·e^(−Δt/τ) + w`, τ≈30d, one column on `recall_stats` + `last_use_at`, O(1) updates. Encodes frequency and freshness-of-use in one number: old-but-used-yesterday memories stay hot; once-popular-but-unused memories cool exponentially.

**Ranking**: `final = rrf + α·log(1+heat) + β·e^(−age/14d)` — both bonuses bounded additive (≤~10% each of median top-1 RRF) so they only break near-ties; relevance always dominates; hot-irrelevant can never outrank cold-relevant. The β term is cold-start insurance: new memories (heat=0) get a small decaying novelty boost for ~14d, then earn their keep.

**Lifecycle coherence**: the same heat drives D2 archival (heat≈0 ∧ 90d unused → doctor proposes). One signal, two consumers: hot → ranks higher; cold → archive candidate.

**Why this beats created_at decay (v2's C2 concern)**: use-based heat refreshes canonical-but-old memories automatically (usage re-heats them), which was the failure mode of pure recency decay.

**Risks & mitigations**: residual feedback loop (agents cat what ranks high) → ≤10% cap + log scale + doctor monitors heat concentration (alarm if top-10 memories eat >50% of cats); thin telemetry (started 2026-08-09) → counter ships first, boost default-on only after ≥30d data + eval gate; skills participate (never-activated skill stays cold; 66-activation skill earns its slot).
