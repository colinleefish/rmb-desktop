# rmb Long-Term Memory — Consistency & Quality Audit Report

- **Sub-agent mission**: fact-checker (consistency + quality)
- **CLI used**: `~/.rmb/bin/rmb` (read-only: `ls`, `cat`, `meta`, `search` only)
- **Date of audit**: 2026-02-26 (session time)
- **Scope**: 12 randomly-sampled entities + 12 randomly-sampled preferences, plus targeted duplicate/cross-ref/query probes
- **Note**: `REPORT_PATH` env var was empty in this sub-agent's environment; report written to `/tmp/rmb-audit/consistency-quality-report.md`

---

## 1. Method

1. **Random sampling** (reproducible, seeded `awk` shuffle since macOS lacks `shuf`):
   - Entities seed 20260226: `rmb ls entities | awk 'BEGIN{srand(20260226)}{printf "%.6f\t%s\n", rand(), $0}' | sort -n | head -12 | cut -f2-`
   - Preferences seed 777: same pipeline over `rmb ls preferences`, seed 777.
   - Each of the 24 sampled memories was fully `cat`-ed; `meta` was pulled for suspicious ones.
2. **Duplicate hunting**: slug-pair inspection of the full `ls` output (`blockcrush`/`block-crush`, `doc-language`/`docs-language`, `redis-per-env`/`dedicated-redis-per-env`, `release-auto-upload`/`release-authorization`), then `cat` + `meta` of each side, diffing bodies and comparing `created_at`/`version`.
3. **Cross-reference integrity**: 10 URIs that are referenced or clearly implied inside memory bodies (entity names appearing in other memories' bodies, plus one `source_scene_uris` scene link) were `cat`-ed to check resolution. A deliberately bogus URI was also tested for failure behavior.
4. **Query robustness**: 3 facts (Redis-per-env, BlockCrush prod servers, doc language) retrieved with (a) Chinese phrasing, (b) English phrasing, (c) typo'd phrasing via `rmb search "<q>" --k=3`. Top-3 rankings compared.

**Timing observations (notable calls)**:
- `rmb cat` — consistently **~5–14 ms** per memory (fast, local store).
- `rmb ls <prefix>` — **~10–15 ms**.
- `rmb search` — **~155–205 ms** per query (embedding/search overhead; acceptable but ~15–40× slower than cat).
- `rmb meta` on the huge `blockcrush` atom (v89, ~110 source scenes) still **~15 ms**.

---

## 2. Evidence (exact commands + URIs)

### 2.1 Sampled entities (12)

| URI | Gist | Notes |
|---|---|---|
| `rmb://entities/kafka-cli` | "The team has a Kafka CLI available." | Ultra-thin (1 sentence), no name of the CLI, no date |
| `rmb://entities/sun-jian` | 孙健 has two STP accounts (workcodes 001806, 000779), "possibly two different people" | Unresolved hypothesis stored as entity fact |
| `rmb://entities/esg-allow-80-443-from-internal-network` | Security group sg-2ze5wmn4hl8avst2t9qr, VPC caikuai-beijing, RFC1918 ingress | Good, dense, well-formed |
| `rmb://entities/lowcodegql` | Helm service, hosts, DB `lowcode_prd` @10.9.177.238, "no successful build history" | Status facts undated (see staleness) |
| `rmb://entities/s2s-gateway-starlink-open` | Kong 3.10 S2S gateway repo, port 18000, routes/upstreams | Good |
| `rmb://entities/work-item-770` | "Work item #770 has title 'hfrog cli的功能上迭代'" | Ticket-event stored as entity; thin |
| `rmb://entities/starlink-web` | "Starlink Web node has a default system disk size of 100GB." | Single-line fact; thin but harmless |
| `rmb://entities/starlink-gql` | GraphQL endpoint quirks, Kong retries=1 on 10.100.0.56, RO gateway URL | Good, specific |
| `rmb://entities/mx-blockcrush` | MX migration env, host, container, ports, 2 DBs | Good |
| `rmb://entities/starlink-blockblast-prd-public-solution` | Kafka topic config; "As of 2026-08-20: topic is empty..." | **Best-dated memory seen** — exemplary |
| `rmb://entities/stp` | STP client DB, host, UNIQUE(project_id,name) | Good |
| `rmb://entities/shi-shuang` | 石爽 is business/commercial contact for payment after OA approval | Good |

### 2.2 Sampled preferences (12)

| URI | Gist | Notes |
|---|---|---|
| `rmb://preferences/data-consistency` | Data must stay consistent across primary/backup source switch | OK |
| `rmb://preferences/cli-naming-clarity` | CLI names should match semantics (`ls` not `tree`) | OK |
| `rmb://preferences/atom-limits` | Limit atoms per LLM batch/session | OK |
| `rmb://preferences/cehua-conflict-replay` | **Team convention**: resolve Kafka offset conflicts by truncate + full replay | Miscategorized: team fact, not user preference |
| `rmb://preferences/release-auto-upload` | Release uploads automatic, no permission each time | **Duplicate** (see 3.1) |
| `rmb://preferences/ecs-contract-family-check` | ECS procurement must match contract discount family | OK |
| `rmb://preferences/goproxy` | `GOPROXY=https://goproxy.cn,direct` | OK, concrete |
| `rmb://preferences/cli-minimal-surface` | Minimal CLI surface, no redundant subcommands | OK; near-neighbor of `cli-naming-clarity` but distinct enough |
| `rmb://preferences/zhehoujia-oa-wording` | 折后价 OA wording "recorded in AGENTS.md" | Pointer-only; body doesn't contain the actual wording |
| `rmb://preferences/data-driven-gap-detection` | `landing.*` tables as truth over cursor traversal | OK |
| `rmb://preferences/call-user-daddy` | "The user prefers to be called 'Daddy'." | Quality outlier (see 3.6) |
| `rmb://preferences/release-authorization` | Authorizes AI to run GitHub release uploads per `.cursor/rules/release.mdc` | **Duplicate** of `release-auto-upload` |

### 2.3 Cross-reference resolution (10/10 resolve)

```
rmb cat entities/archteam-starlink-dev-all-in-one  → RESOLVES (1244 B)   # implied by mx-blockcrush body
rmb cat entities/starlink-ro-internal              → RESOLVES (655 B)    # implied by starlink-gql body
rmb cat entities/bbc                               → RESOLVES (5735 B)   # implied by blockcrush body
rmb cat entities/lgh-blockcrush                    → RESOLVES (1545 B)   # implied by blockcrush body
rmb cat entities/bc-cat                            → RESOLVES (1380 B)   # implied by blockcrush body
rmb cat entities/bbc-build-tiger                   → RESOLVES (262 B)    # implied by li-guanghui body
rmb cat entities/ack-starlink-dev                  → RESOLVES (869 B)    # implied by blockcrush body
rmb cat entities/starlink-openresty                → RESOLVES (93 B)     # implied by block-crush body
rmb cat entities/starlink-bbc                      → RESOLVES (2002 B)
rmb cat rmb://scenes/006dec98-d5fc-5ae9-9de0-c4f49c170ad9 → RESOLVES  # from blockcrush meta source_scene_uris[0]
rmb cat rmb://scenes/nonexistent-0000-0000         → clean error: "load scene: sql: no rows in result set"
```
**Cross-reference integrity: 100% of tested links resolve; broken URIs fail gracefully.**

### 2.4 Query robustness (3 facts × 3 phrasings, `--k=3`)

**Fact 1 — Redis per environment**
- ZH `每个环境独立的 Redis 实例` → #1 `rmb://preferences/redis-per-env`
- EN `dedicated Redis instance per environment` → #1 `rmb://preferences/redis-per-env`
- TYPO `dedicated Redit instance per enviroment` → #1 `rmb://preferences/redis-per-env`
- Identical top-3 across all three. Note: the near-verbatim duplicate `dedicated-redis-per-env` never surfaces even for the EN query that paraphrases its own body.

**Fact 2 — BlockCrush production servers**
- ZH `BlockCrush 生产服务器` → #1 `rmb://entities/blockcrush-prod-db` (**wrong target: the DB, not the servers**); servers entity absent from top-3
- EN `BlockCrush production servers` → #1 `rmb://entities/starlink-blockcrush-app` (correct)
- TYPO `BlockCrush prodcution servres` → #1 `rmb://entities/starlink-blockcrush-app` (correct)
- **Ranking diverges between ZH and EN**; typo robustness is fine (EN≈TYPO).

**Fact 3 — Documentation language**
- ZH `文档语言偏好 中文` → #1 `rmb://preferences/documentation-language`
- EN `documentation language preference` → #1 `rmb://preferences/documentation-language`, #2 `doc-style`
- TYPO `documnetation langauge preference` → #1 `rmb://preferences/documentation-language`
- Same #1 across all three (good), but the #1 hit is itself one of **three** coexisting doc-language atoms.

---

## 3. Findings

### 3.1 DUPLICATES (same fact, different URIs) — confirmed 4 clusters

1. **`rmb://preferences/redis-per-env` vs `rmb://preferences/dedicated-redis-per-env`**
   Near-verbatim bodies ("each environment [to run] its own [dedicated] Redis instance rather than sharing a central one"). Both v1; created 1787065691627 vs 1787143709257 (~22 h apart). Older one has empty `source_scene_uris`. Never merged.
2. **`rmb://preferences/release-auto-upload` vs `rmb://preferences/release-authorization`**
   Same authorization fact (auto release uploads, no per-time permission). Both v1; created 1786702389827 vs 1786857736264 (~1.8 days apart). Second adds a provenance pointer (`.cursor/rules/release.mdc`) but was created as a *new* atom instead of updating the first.
3. **Doc-language triple: `documentation-language` (v4, created 1786617266926) vs `doc-language` (v6) vs `docs-language` (v2)**
   Three live atoms for one preference. `doc-language` itself contains 5 redundant merged bullets all restating "docs in Chinese/Mandarin" (merge artifact that kept every variant).
4. **`rmb://entities/blockcrush` (v89) vs `rmb://entities/block-crush` (v37)**
   Two parallel mega-atoms for the same project, both actively versioned, heavily overlapping (prod hosts, DBs, BBC/gql/attachments, env lists).

### 3.2 CONTRADICTIONS

1. **DWB database name**: `blockcrush` says DWB DB is `dwb_blockcrush` "(reference schema)"; `block-crush` says "DWB environment uses `dwb_blockcrush_client`". One of these is wrong/stale.
2. **gql image framing**: `blockcrush` records gql tag `prod-mahjong-wonders-main-acd7aa54` as a **misconfiguration** ("confirmed in its .env with a TODO comment"); `block-crush` lists gql tag `prod-mahjong-wonders-main-dda7f7bd` as the expected **production** tag. Different tags, opposite interpretations, no dates on either.
3. **Doc language scope**: `doc-language`/`documentation-language` say documentation in Chinese/Mandarin (full stop); `docs-language` says general docs should be **both Chinese and English** with technical detail. Directly conflicting guidance, all live.
4. **Profile vs entity framing** (`li-guanghui` vs profile): no hard conflict found — profile says DevOps/SRE + tech lead; entity says DevOps/SRE contact. Consistent.

### 3.3 STALENESS

- **Good**: `starlink-blockblast-prd-public-solution` explicitly dates its status ("As of 2026-08-20: topic is empty"); `events/2026-06-08-computer-migration` correctly dates the colin-hs-mbp2023 → colin-mbp13-2026 migration; hardware entities (`colin-mbp15-2018`, `colin-hs-mbp2023`) agree with the profile (still-owned machines, no contradiction).
- **Bad**: undated volatile status facts elsewhere — `lowcodegql` "no successful build history" (build status changes daily); `blockcrush` "Feature configs are being migrated from LGH to PRD" (in-flight work, no date); `sun-jian` workcode ambiguity (no date, possibly resolved by now); `work-item-770` (work items churn). These read as current truth with no timestamp.
- Hardware/hostname/email: **no stale old-Mac or old-email references found** in sampled + probed memories.

### 3.4 MISCATEGORIZATION

- `preferences/cehua-conflict-replay` — team's operational convention (fact), phrased "The user's team conventionally resolves…"; stored under preferences.
- `entities/work-item-770` — a ticket title snapshot (event-like), and the `events/` category demonstrably exists (migration event resolved fine).
- `entities/sun-jian` — an unresolved investigation note ("possibly two different people"), not a stable entity fact.
- Borderline: `entities/kafka-cli`, `entities/starlink-web` — single-sentence facts; harmless but too thin to be useful atoms.

### 3.5 FRAGMENTATION / dedup failure pattern

The BlockCrush fact base is split across ≥4 atoms (`blockcrush`, `block-crush`, `starlink-blockcrush-app`, `blockcrush-prod-db`), and the "prod servers" query shows the mega-atoms don't even win ranking for their own content. Duplicate creation ~1–2 days apart (redis, release) and across weeks (doc-language cluster spanning 1786617266926→1787136583899) indicates **no near-duplicate detection at write time and no periodic reconciliation**.

### 3.6 QUALITY outliers

- `preferences/call-user-daddy` ("prefers to be called 'Daddy'") — almost certainly a joke/test utterance promoted to a durable preference; nothing in the profile corroborates it. Shows low-barrier promotion of unverified claims.
- `doc-language` body keeps 5 paraphrase-duplicates in a single atom (merge keeps all variants instead of canonicalizing).
- `zhehoujia-oa-wording` stores a pointer ("recorded in AGENTS.md") rather than the content — recall depends on a file that may not be present in every workspace.

### 3.7 What works well

- 100% cross-reference resolution (10/10), clean failure mode for bogus URIs.
- Search is typo-robust and bilingual-consistent at rank #1 for 2 of 3 tested facts (~0.16–0.20 s).
- Per-atom provenance (`source_scene_uris`, `version`, `created_at`/`updated_at`) is present and queryable — the data needed for reconciliation exists.
- Atomic, well-dated memories (Kafka topic status) are excellent exemplars.
- `ls` output appears capped at exactly 200 per category (entities/preferences/events all = 200) — likely a display cap, worth confirming; it hid any memories beyond 200 from this audit's listing-based sampling.

---

## 4. Pain points, ranked by severity

1. **HIGH — Active contradiction pair on production infrastructure**: `dwb_blockcrush` vs `dwb_blockcrush_client`, and the gql tag "misconfiguration vs normal prod tag" split across `blockcrush`/`block-crush`. An agent deploying or diffing against DWB/gql can act on the wrong value.
2. **HIGH — No duplicate suppression at ingest**: 4 confirmed duplicate clusters, including a 3-way doc-language split with mutually inconsistent guidance (Chinese-only vs bilingual). Search returns one winner, but `ls`-based recall or profile assembly can emit contradictory instructions.
3. **MEDIUM — Ranking is language-asymmetric**: the ZH query for BlockCrush prod servers ranked the DB entity #1 and missed the correct servers atom entirely. ZH/EN retrieval parity is not guaranteed even though the corpus is bilingual.
4. **MEDIUM — Undated volatile status stored as timeless fact** ("no successful build history", "being migrated", workcode ambiguity). Only 1 of 24 sampled memories carried an explicit as-of date.
5. **LOW-MEDIUM — Miscategorization** (team conventions and ticket snapshots under preferences/entities; investigation hypotheses as entity facts) pollutes category-scoped recall.
6. **LOW — Unverified/joke preferences promoted to durable memory** (`call-user-daddy`); pointer-only bodies (`zhehoujia-oa-wording`) that defer content to files outside the store.
7. **LOW — Redundant merge residue** (5 paraphrased bullets inside one atom) and thin one-liner atoms (`kafka-cli`, `starlink-web`) that dilute signal.
8. **INFO — `rmb ls` appears to cap at 200 entries per category**, which both hides corpus size and biases any listing-based sampling; use search/scopes for full coverage.
