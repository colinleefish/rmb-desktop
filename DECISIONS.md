# DECISIONS.md

One line per resolved decision. Pointer = where the full rationale lives. Append-only; supersede with a new line, never edit history.

| Date | Decision | Pointer |
|---|---|---|
| 2026-08-15 | Replace Tauri shell with pure-Go tray app (D1–D4: zip format · ed25519 now · v0.2.0 · split status/Open Dashboard) | `plan/done/tauri-to-go-shell.md` |
| 2026-08-22 | Memory-primary default search scope; evidence reachable by drill-down, not sprayed into rankings | `plan/memory-retrieval-remediation.md` §6 D1 |
| 2026-08-22 | Archive threshold = 90 days, doctor-proposed, user-approved, reversible; evidence tiers exempt | §6 D2 |
| 2026-08-22 | Usage-heat ranking (not raw recency decay), phased default-on after eval gate | §6 D3 |
| 2026-08-22 | Contradiction evidence gathered by sub-agent with jump-hs99-vip skill, read-only | §6 D4 |
| 2026-08-22 | doc-language canonical = `docs-language` (中文 technical + bilingual general); supersede the other two | §6 D5 |
| 2026-08-22 | Skills tier = FTS-only + capped at 1 slot in default scope (lexical signal is correct for skills) | remediation plan §4 P0.2 |
| 2026-08-22 | Retrieve-then-canonicalize at distill time replaces fuzzy slug matching | §4 P2.1 |
| 2026-08-24 | Harness refactor per learn-harness-engineering five-subsystem framework; milestone #5, issues #53–#58 | milestone harness-ready |
| 2026-08-24 | `make check` (vet+build+test+eval) is the repo's consistent-state predicate | AGENTS.md, #55 |
| 2026-08-24 | Plans move to `plan/done/` when complete (tauri-to-go-shell first) | repo convention |
| 2026-08-28 | ZCode added as a 7th supported agent integration; config lives at `~/.zcode/cli/config.json` (`hooks.events.Stop`, forces `hooks.enabled: true`), recall block at `~/.zcode/AGENTS.md` | feature_list.json F07 |
| 2026-08-29 | #62 ZCode session keys: normalize at parse time only (`zcodeRMBSessionID`); NO backfill migration — owner is the sole ZCode user, 3 legacy `sess_…` rows accepted as orphaned | `internal/hook/zcode.go`, `docs/audit/2026-08-29-zcode-capture-bugs/INVESTIGATION.md` §5.1 |
| 2026-09-12 | Pre-F08 merged bugs (#61/#62 → B01/B02): administrative backfill to `passing` when the GitHub issue is closed and the L2 regression passes on main; skip the `diagnosed` gate (regression is already green) | `chore/bug-track-hygiene`, `bug_list.json` |
