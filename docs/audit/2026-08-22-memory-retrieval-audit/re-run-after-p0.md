# E2E re-run — after P0 + P1.1 (vs audit baseline)

Date: 2026-08-22 (same day as the original audit). CLI/daemon: NEW build
(`chore/code-grooming` @ 399e396, installed as live) against the **real** db at
19019. Same 8 queries + 2 adversarial probes as `treasure-hunter-retrieval-stress-test.md`.
Default scope is now memory-only (scenes/skills out by design, D1/#19); depth is
reachable via `--scope=atom`/`--scope=scene` and recency via `--since`.

## Per-query before → after

| # | Query | Audit baseline | After P0+P1.1 |
|---|-------|---------------|---------------|
| Q1 | cluster-admin-toolbox rejected/removed | rank3 harmful `[skills] herdr`; scene dup at #2 | clean: removal #1, reject #2; **no scene, no skill noise** |
| Q2 | bbc-deploy-k8s-param-split | event buried at #4 (entity+scene above) | event **#2** directly (entity #1) |
| Q3 | A股 首板 量价 | need 2 cats: preference + scene for Pattern A | all-memory, `stock-first-board-filter` #1; Pattern A now an entity | 
| Q4 | HEIC Nikon Z30 | workflow query→identity scenes; reassemble ≥3 items | naming rule #2, sync events #3/#4, no scene noise |
| Q5 | why sqlite (hard) | took **5 reformulations** to find rationale (in a scene) | default still returns ops-fixes (rationale lives in a scene by design); `--scope=scene` surfaces `cdccc2a4` decision **#1 in one shot** |
| Q6 | SSH bastion jump.hs99.vip | entity+scene+skill auto-surfaced | entities #1/#2 + pref #3 (skill now explicit via `--scope=skill`, D1) |
| Q7 | 删除标签 怎么解决 | unanswerable (4 incidents, no resolution) | still open — **resolution/outcome arcs are an ingest gap (P2 #28/#29), not P0** |
| Q8 | starlink openresty dns | "resolver IPs/config not in memory" — dead-end | memory one-liners, but `--scope=atom` now returns resolver/dynamic-DNS detail + scene/session drill-down (**#23**) |
| A | toolboks removd (typo) | good | still good: correct hit #1 |
| B | 堡垒机/跳板机 (CH↔EN) | good | still good: jumpserver #1/#2 |

## Findings delta (from audit §"Findings")

1. **Skill pollution — FIXED.** `draft-aliyun-procurement-ticket` no longer floods
   default results (was top-5 in ~6/14 searches). Skills are now FTS-only + capped
   (#19); `herdr` no longer rank-3 on unrelated queries.
2. **Event/scene duplication — FIXED.** Scenes are out of default scope, so
   byte-identical event/scene text no longer occupies multiple ranks (#19).
3. **Rationale ("why") missing — PARTIAL.** Still absent from default memory hits
   (rationale lives in scenes); but explicit `--scope=scene` recover it decisively
   (Q5 trained #1). Full fix (distill decision records) is P2 (#28).
4. **Resolution/outcome arcs ("怎么解决") missing — UNCHANGED.** Q7 still
   unanswerable. Ingest gap → P2 (#28 distill why/outcome, #29 reconciliation).
5. **One-sentence bodies cap depth — IMPROVED.** `--scope=atom` (#23) recovers the
   underlying detail (Q8 resolver atoms) with scene/session drill-down annotations.
6. **"Latest" — IMPROVED.** `--since=7d`/`--until` now filter search by time (#20);
   `ls` prefix+pagination (#18) still the timeline browse.

## Remaining pain (not fixed by P0/P1.1 — mapped to later phases)

- Distillation still stores actions, not rationales/outcomes → **P2 #28, #29**
- Duplicate facts across tiers still exist *at ingest* (dedup is display-level now) → **P2 #27**, **P1.4 #26** (cross-tier suppression, not yet merged)
- Heat-based ranking + telemetry → **P1.2 #24 / P1.3 #25** (not yet merged)

## Verdict

The user-facing retrieval *surface* is materially cleaner: no skill/scene pollution,
no duplicate flooding, direct hits at top-1/2, version trust signal, and two new
escape hatches (`--scope=atom` for depth, `--scope=scene` for rationale,
`--since` for recency). The remaining gaps are **content/ingest quality** (P2),
not retrieval plumbing — exactly as the plan's phase split anticipated.
