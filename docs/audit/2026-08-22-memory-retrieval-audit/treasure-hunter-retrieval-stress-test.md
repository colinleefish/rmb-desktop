# Mission: treasure-hunter — rmb retrieval stress test

Date: 2026 session (audit). CLI: `~/.rmb/bin/rmb` (strictly read-only: `search` / `ls` / `cat` / `meta` only).
Scope: 8 realistic user questions + 2 invented adversarial queries. No scores are shown by `rmb search` (rank order only), so results below are listed in ranked order.

## Method

1. Ran `~/.rmb/bin/rmb` (no args) and `rmb cat rmb://agent` to load the recall rules and URI shapes (~instant, 0.02s).
2. For each of the 8 questions: issued the most natural first query (usually mirroring the user's own phrasing, including Chinese where the user would use Chinese), then `rmb cat` on the top hits until the question was answered or clearly unanswerable.
3. Logged exact query strings, ranked results (uri + tier), number of follow-up cats, confidence, and friction.
4. Two adversarial probes at the end: a typo'd query and a Chinese↔English vocabulary-mismatch query.
5. CLI latency noted via `time` for every notable call.

Latency summary: every `search` returned in **0.16–0.26s**; every `cat` in **0.005–0.02s**; `rmb --help`/`rmb cat rmb://agent` ~0.02s. Performance is a non-issue.

---

## Evidence per question

### Q1. What happened with the cluster-admin-toolbox? Why was it rejected/removed?

Queries tried:
1. `~/.rmb/bin/rmb search "cluster-admin-toolbox rejected removed"` (0.22s)

Top results (rank order, no scores shown):
1. `[memories] rmb://events/2026-08-21-cluster-admin-toolbox-removal` — "On 2026-08-21, the cluster-admin toolbox was removed from the cluster and local shared/deploy-shell files were deleted."
2. `[scenes] rmb://scenes/1c572bfd-f6b1-576d-a593-a35ca22915c2` (duplicate text of #1)
3. `[skills] rmb://skills/herdr` (irrelevant)
4. `[memories] rmb://events/2026-08-21-reject-cluster-admin-toolbox` — "user rejected the cluster-admin toolbox approach for deployment"

Follow-up cats: 3 (`cat` on events/...-removal, events/...-reject-..., scenes/1c572bfd...).

Confidence: **Partial.** WHAT happened is crisply recalled (rejected on 2026-08-21, removed from cluster + local shared/deploy-shell files deleted). But **WHY** is not recorded anywhere — both the event and the scene carry the identical one-line text with no rationale. Annoyance: the rejection event and the removal event are near-duplicates; neither has a reason, and `cat` on the scene added zero information over the event.

### Q2. What was the BBC deploy k8s parameter split about (bbc-deploy-k8s-param-split)?

Queries tried:
1. `~/.rmb/bin/rmb search "bbc-deploy-k8s-param-split"` (0.20s)

Top results:
1. `[memories] rmb://entities/bbc-deploy-k8s` — Jenkins deploy job, copyArtifacts from bbc-build-tiger, K8s/helmfile, Jenkinsfile path.
2. `[scenes] rmb://scenes/d667d9da-5450-5b1a-b295-1c763876e384` — ack-starlink-prod / Helmfile context.
4. `[memories] rmb://events/2026-08-21-bbc-deploy-k8s-param-split` — "Jenkinsfile updated to use separate GAME and ENV parameters, combining as DEPLOY_ENV = GAME/ENV."

Follow-up cats: 2 (`events/2026-08-21-bbc-deploy-k8s-param-split`, `entities/bbc-deploy-k8s`).

Confidence: **Yes.** Exact-match slug query put the event at rank 4 (entity above it, which is fine). Answer: on 2026-08-21 the Jenkinsfile switched from a single env param to separate `GAME` + `ENV` params, combined as `DEPLOY_ENV = GAME/ENV`. Minor friction: the event is one line — no mention of why the split was needed or which games it enables (the rank-2 scene about the earlier build #5 crash is adjacent history, not the split's motivation).

### Q3. What is the user's A-share (A股) first-board (首板) strategy entry rule?

Queries tried:
1. `~/.rmb/bin/rmb search "A股 首板 入选条件 量价"` (0.19s)
2. `~/.rmb/bin/rmb search "first board entry rule strategy A-share"` (0.17s) — deliberate English re-phrasing to test cross-lingual robustness

Top results (query 1):
1. `[memories] rmb://preferences/stock-first-board-filter` — 排除前期大涨/已涨停股，只选底部启动首板，结合完整波浪结构与历史涨停记录
2. `[scenes] rmb://scenes/6257f09a-...` — first Friday-close funnel analysis on 819 stocks, 2026-07-10
4. `[memories] rmb://events/2026-07-10-kechuan-first-board` — 中国科传 first board
5. `[scenes] rmb://scenes/d0e388d6-9914-5a7b-ad37-fb20d56351cb` — Pattern A / A+ / B / CTRL definitions

The English query also surfaced the right cluster (rank 1: `preferences/second-wave-screening`, rank 5: scene "interested in first-board strategies") — cross-language retrieval works.

Follow-up cats: 2 (`preferences/stock-first-board-filter`, `scenes/d0e388d6...`).

Confidence: **Yes, strongly.** Entry rule fully recovered: exclude stocks with prior big runs/prior limit-ups, only bottom-start first boards, consider full wave structure + limit-up history; quantitatively: Pattern A = limit-up with turnover < 3%, 20-day return in [-15%, +5%], first limit-up within 10 days; Pattern A+ adds market cap 80–220亿, PE 18–45, PB < 3.5. Annoyance: the rules are split across a preference and a scene; you need both cats to assemble the full answer.

### Q4. What is the user's photo workflow (HEIC, Nikon Z30)?

Queries tried:
1. `~/.rmb/bin/rmb search "HEIC Nikon Z30 photo workflow"` (0.17s)
2. `~/.rmb/bin/rmb search "photo import workflow convert HEIC jpg pipeline"` (0.21s)

Top results (query 1):
1. `[scenes] rmb://scenes/8c533793-...` — Nikon Z30 + iPhone, ~4,000+ HEIC photos in six months
5. `[memories] rmb://preferences/nikon-naming-rule` — rename pattern `yyyymmddhhmmss.<short-desc>.<md5[0:8]>.jpg`

Query 2 added: `events/2026-06-08-trip-subsets-not-uploaded` (only HEIC and MOV folders synced to photo-iphone13mini; trip subsets local-only).

Follow-up cats: 2 (`preferences/nikon-naming-rule`, plus skimming the sync event snippet).

Confidence: **Mostly.** Naming rule is fully recovered (timestamp + vision-generated kebab-case description + MD5 prefix). Sync behavior partially recovered. Annoyance: there is no single consolidated "photo workflow" memory — the pipeline (import → convert → describe → rename → sync targets) must be reassembled from ≥3 scattered scenes/preferences; the "workflow" query itself returned mostly identity-flavored scenes ("user is a photography enthusiast") rather than process detail.

### Q5. Why did rmbd choose sqlite? And what was the duckdb-to-postgres move for PBP?

Queries tried:
1. `~/.rmb/bin/rmb search "rmbd why sqlite storage choice"` (0.18s) — surfaced fixes, not the decision
2. `~/.rmb/bin/rmb search "duckdb to postgres migration PBP"` (0.18s) — direct hit
3. `~/.rmb/bin/rmb search "rmbd chose sqlite reason embedded"` (0.23s) — still fixes
4. `~/.rmb/bin/rmb search "sqlite single file no server local daemon decision"` (0.19s) — noisy
5. `~/.rmb/bin/rmb search "why sqlite instead of postgres for rmb daemon"` (0.16s) — **hit at rank 2**

Key finds:
- `[scenes] rmb://scenes/cdccc2a4-f2c5-5fa3-bd3e-a111ef85c229` (found on query 5): "chose SQLite as its database engine to allow easy installation without RDS, Docker, or port"; single daemon rmbd with app-level mutex, WAL, BEGIN IMMEDIATE.
- `[memories] rmb://events/duckdb-to-postgres-pbp` — "The user migrated from DuckDB to Postgres on PBP." (one line)
- `[memories] rmb://events/2026-07-11-pbp-to-postgres` — "On 2026-07-11 PBP migrated from DuckDB/OSS Parquet to Postgres (pbp_db)." (one line)

Follow-up cats: 5 (`events/duckdb-to-postgres-pbp`, `events/2026-07-11-pbp-to-postgres`, `scenes/cdccc2a4...`).

Confidence: **Yes for rmbd-sqlite** (easy install, no RDS/Docker/port exposure; single daemon + WAL + BEGIN IMMEDIATE) — but it took **5 reformulations** to find the rationale; naive phrasings returned the *operational fixes* (`rmbd-txlock-immediate-fix`, `rmbd-concurrency-defaults-raise`) rather than the *decision record*. **Partial for PBP**: the fact and date of the DuckDB→Postgres (pbp_db) migration are recorded, but the motivation (why leave DuckDB/OSS Parquet?) is absent, and two near-duplicate events exist for it.

### Q6. How does the user SSH through the jump.hs99.vip bastion?

Queries tried:
1. `~/.rmb/bin/rmb search "SSH bastion jump.hs99.vip ProxyJump"` (0.19s)

Top results:
1. `[memories] rmb://entities/jumpserver`
2. `[scenes] rmb://scenes/e7da4ea8-...`
3. `[skills] rmb://skills/jump-hs99-vip` (USER/OPERATOR skill)
4. `[memories] rmb://entities/jump-hs99-vip`
5. `[scenes] rmb://scenes/86ab1f19-...` — KoKo SSH port 2222, osadmin default, proxy.hungrystudio.pp.ua:1080

Follow-up cats: 1 (`cat rmb://entities/jump-hs99-vip`).

Confidence: **Yes, excellent — best result of the audit.** Full mechanics in one cat: https://jump.hs99.vip, KoKo SSH gateway on port 2222, auth as `JMS-<token_id>` / `<token_value>` from conn_token.py, tokens single-use expiring ~5 min (reuse → Permission denied), SCP/SFTP don't work → tar-over-SSH, default asset account `osadmin`, API creds at ~/.hungrystudio/jump-hs99-vip, plus a dedicated skill `rmb://skills/jump-hs99-vip`. Entity + scene + skill reinforce each other instead of conflicting.

### Q7. (vague Chinese) 那个删除标签的问题最后怎么解决的？

Queries tried:
1. `~/.rmb/bin/rmb search "删除标签 标签问题 解决"` (0.21s)
2. `~/.rmb/bin/rmb search "tag 删除 删标签 bug"` (0.20s)

Top results (query 1):
1. `[memories] rmb://events/2026-07-16-soft-delete-one-tag-solutions` — soft-deleted 29,800 of 29,867 one-tag-diff solutions in lhh_blockblast_client, leaving 67
2. `[scenes] rmb://scenes/d3181457-...` — 罗弘 edits feature_tag_config tags (different topic)
4. `[memories] rmb://events/2026-07-13-starlink-hs99-vip-500-bug` — get_diff_tag_from_tag_base_editer dict-vs-list bug
5. `[scenes] rmb://scenes/ffd845e6-...` — rows 261763/261764 deleted from yunxiao.one_tag_solution_yunxiao_info on 2026-08-21

Follow-up cats: 2 (`events/2026-07-16-soft-delete-one-tag-solutions`, skim of scene d3181457).

Confidence: **No — the honest failure case.** Retrieval is fine (several plausible "tag deletion" incidents surfaced), but the memory content doesn't disambiguate: at least 4 distinct incidents involve deleting tags/solutions, and none of them records a "问题最后怎么解决" resolution arc. A vague follow-up question to the user would be unavoidable. Annoyance: events record the *action* but not the *resolution* or *outcome*, which is exactly what vague follow-up questions ask for.

### Q8. (ambiguous) starlink openresty — dynamic DNS solution details

Queries tried:
1. `~/.rmb/bin/rmb search "starlink openresty dynamic dns"` (0.24s)

Top results:
1. `[memories] rmb://events/2026-07-28-openresty-dynamic-dns-investigation` — investigation saved to `20260728.starlink-openresty-dynamic-dns-investigation/`
2. `[scenes] rmb://scenes/a391a8fa-...` — "Ding uses static upstreams.conf files per deployment namespace. On 2026-08-21, the starlink openresty config was updated to use request-time DNS with a resolver..."
3. `[memories] rmb://events/2026-08-21-starlink-openresty-dynamic-dns` — "updated to use request-time DNS with a resolver for dynamic service discovery, resolver IPs verified against the ack-starlink-dev cluster"

Follow-up cats: 2 (`events/2026-08-21-starlink-openresty-dynamic-dns`, `scenes/a391a8fa...`).

Confidence: **Partial.** The shape of the solution is recovered: moved from static per-namespace `upstreams.conf` to request-time DNS with an explicit `resolver` directive (resolver IPs verified against ack-starlink-dev). But the actual details a user would want — the resolver IPs themselves, the config snippet, how upstreams are now expressed — are not in memory; the 2026-07-28 event only points at a dated local task folder (`20260728.starlink-openresty-dynamic-dns-investigation/`) with no content stored. One-line summaries again cap the depth.

### Bonus probe A — typo'd query (invented)

Query: `~/.rmb/bin/rmb search "cluster-admin-toolboks removd"` (0.24s)

Results: rank 1 = `rmb://events/2026-08-21-cluster-admin-toolbox-removal`, rank 2 = the same-text scene — i.e. **typo tolerance works perfectly**; the correct answer stayed at rank 1 despite two typos (toolboks, removd). Ranks 3–5 were noise (`skills/herdr`, an unrelated `aidaily-removed-prod` event, a generic kubeconfig scene), acceptable.

### Bonus probe B — Chinese↔English vocabulary mismatch (invented)

Query: `~/.rmb/bin/rmb search "堡垒机怎么连服务器 跳板机"` (0.19s) — Chinese terms (堡垒机/跳板机) for a fact stored in English ("JumpServer bastion").

Results: rank 1 = `rmb://entities/jumpserver`, rank 3 = `rmb://skills/jump-hs99-vip`, rank 5 = the port-2222 scene — **cross-lingual retrieval is solid**. English queries against Chinese-stored stock facts also worked in Q3. No language trap found.

---

## Findings

1. **Precision is high for named things.** Slug-style and entity-style queries (bbc-deploy-k8s-param-split, jump.hs99.vip, HEIC Nikon Z30) hit the right memory at rank 1–4 nearly every time; the entity+scene+event redundancy usually converges on the same fact.
2. **Typo and cross-language robustness is genuinely good.** Both adversarial probes returned the correct top-1 without reformulation.
3. **Rationale is systematically missing ("what", not "why").** Q1 (why rejected), Q2 (why split params), Q5 (why DuckDB→Postgres for PBP) all record actions without motivations. Only rmbd's sqlite choice kept its rationale, and only inside a *scene*, not an event.
4. **Resolution/outcome arcs are missing.** Q7's "最后怎么解决的" is unanswerable for any candidate incident — events log the action taken, never the verification/aftermath.
5. **Event bodies are one-liners.** Distilled events are single sentences; scenes only occasionally richer (Pattern A rules, jump-hs99-vip entity being the standouts). Depth beyond the summary requires catting the scene and hoping it has bullets.
6. **Duplicates are common.** events and scenes frequently carry byte-identical text (cluster-admin-toolbox, openresty, pbp-to-postgres×2), inflating result lists without adding information and occasionally pushing unique content below rank 5.
7. **Skills pollute results.** `rmb://skills/draft-aliyun-procurement-ticket` appeared in the top-5 of at least 6 of ~14 searches, always irrelevant. Something is wrong with its embedding/description ranking.
8. **"Latest" must come from ls, not search** — as documented in rmb://agent; search ranking ignores time (both the 2026-07-28 investigation and the 2026-08-21 fix co-ranked in Q8).
9. **Performance is excellent.** searches 0.16–0.26s, cats 0.005–0.02s, no scope filters ever needed for latency.
10. **Pointers to external artifacts are not followed up.** The 2026-07-28 investigation event points at a dated local folder; memory stores the pointer but none of the findings, so "details" questions dead-end.

## Pain points, ranked by severity

1. **(High) No why/resolution capture in distilled events.** The single most user-hostile gap: follow-up questions ("why was it rejected?", "怎么解决的?") — precisely the questions long-term memory exists for — return facts without rationales or outcomes (Q1, Q2, Q5-PBP, Q7). Fix: distillation should extract decision rationale and final outcome, not just the action.
2. **(High) Vague Chinese queries can't be disambiguated from stored content.** Q7 surfaced 4 plausible incidents with no way to tell which is "那个问题". Fix: richer scene bodies (ticket link, product name, resolution) would let the agent self-disambiguate instead of bouncing back to the user.
3. **(Medium) Irrelevant skill (`draft-aliyun-procurement-ticket`) ranks top-5 almost universally.** Wastes 1–2 of 5 result slots on every query. Fix: tune skill ranking/description embedding, or demote skills that never get activated.
4. **(Medium) Event/scene duplication inflates results.** Identical text at multiple ranks crowds out distinct memories near the rank-5 cutoff. Fix: dedupe identical bodies in search results (show one, tag the rest).
5. **(Medium) One-sentence memories can't answer "details" questions.** Q8 (openresty resolver specifics) and Q4 (photo pipeline) dead-end or require reassembly from 3+ cats. Fix: allow events to carry a short "details/refs" block (resolver IPs, config snippet keys, folder paths with a one-line content summary).
6. **(Low) Rationale locked in scenes, not events.** The sqlite rationale (scene cdccc2a4) took 5 query reformulations because operational fix-events outranked the decision scene for every natural "why sqlite" phrasing. Fix: also distill decision records as `events/<date>-decision-*` with rationale in the body.
7. **(Low) No scores shown in search output.** Rank-only output makes it hard for an agent to distinguish "confident hit" from "weak hit" without catting. Fix: show a similarity score per result.
