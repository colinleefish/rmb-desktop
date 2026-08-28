# PROGRESS.md

## Current State

Last commit: d14cadd (base) + zcode-integration work on `feat/zcode-integration` (not yet merged) | `make check`: passing | F07 (zcode-agent-integration): passing, evidence commit d14cadd 2026-08-28.

## In Progress

Nothing — F07 (ZCode agent integration) complete on branch `feat/zcode-integration`, worktree `../rmb-desktop-zcode`. Needs review + merge into main.

## Next Steps

1. Merge `feat/zcode-integration` into main (merge commit, per `plan/parallel-work-and-versioning.md`), delete branch/worktree after.
2. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first — `make verify-feature F=F03 A=1`, write `docs/audit/webui-ux-audit.md`, then F04 shell restructure, F05 settings strangler
3. F06 secrets → Keychain/env + key rotation (P3.5, last open item from `plan/memory-retrieval-remediation.md`)
4. Weekly sweep per AGENTS.md Observability: re-run `temp/learn-harness-engineering/tools/audit-harness.sh .`, re-score `docs/quality-document.md`, promote review findings into `.harness/arch-rules.json`

## Blockers

None. Note: F07's ZCode Stop-hook payload shape was inferred from the locally installed
`zcode-guide` marketplace plugin skills (`zcode-configuration-guide`, `diagnosing-hooks`)
plus the user-provided ZCode hooks documentation, not from a captured live Stop hook
payload — no live ZCode session with `hook_event_name` was found under `~/.zcode/cli` at
implementation time (only subagent transcripts, which don't fire Stop hooks). If a real
ZCode Stop payload turns out to differ from the assumed Claude-compatible shape
(session_id/transcript_path/cwd/last_assistant_message/stop_hook_active), revisit
`internal/hook/zcode.go`.
