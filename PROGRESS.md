# PROGRESS.md

## Current State

Last commit: 6ef9031 (main) — merged `feat/zcode-integration` (F07) via merge commit 9b70cb3, bumped version in 6ef9031, bumped VERSION to 0.2.10-dev.1, rebuilt + reinstalled `/Applications/RMB Desktop.app` and `~/.rmb/bin/{rmb,rmbd-desktop,rmb-app}` locally (ad-hoc signed), applied the ZCode hook + recall block on this machine. `make check`: passing. F07: passing, evidence commit d14cadd base + 80006f0. All 7 agents now show hook=configured recall=configured via `rmb setup status`.

## In Progress

Nothing. Branch `feat/zcode-integration` merged, deleted (local + remote), worktree removed.

## Next Steps

1. WebUI refactor per `plan/webui-refactor.md` (week of 2026-08-31): F03 UX audit first — `make verify-feature F=F03 A=1`, write `docs/audit/webui-ux-audit.md`, then F04 shell restructure, F05 settings strangler
2. F06 secrets → Keychain/env + key rotation (P3.5, last open item from `plan/memory-retrieval-remediation.md`)
3. Weekly sweep per AGENTS.md Observability: re-run `temp/learn-harness-engineering/tools/audit-harness.sh .`, re-score `docs/quality-document.md`, promote review findings into `.harness/arch-rules.json`
4. When ready for a real GitHub release, run `make release VERSION=x.y.z` with `SIGN_KEYCHAIN_PASS` (this session only did a local ad-hoc-signed install, not notarize/upload — keychain password wasn't available).

## Blockers

None. Note: F07's ZCode Stop-hook payload shape was inferred from the locally installed
`zcode-guide` marketplace plugin skills (`zcode-configuration-guide`, `diagnosing-hooks`)
plus the user-provided ZCode hooks documentation, not from a captured live Stop hook
payload — no live ZCode session with `hook_event_name` was found under `~/.zcode/cli` at
implementation time (only subagent transcripts, which don't fire Stop hooks). If a real
ZCode Stop payload turns out to differ from the assumed Claude-compatible shape
(session_id/transcript_path/cwd/last_assistant_message/stop_hook_active), revisit
`internal/hook/zcode.go`.
