# Quality Document — module health scores

> New sessions read this to know where to be careful. Updated at clock-out for touched modules
> (AGENTS.md Clock-out). Grades A (strong) → D (debt). Scores are honest opinions backed by
> evidence pointers, not vanity metrics.

| Module | Tests | Complexity | Docs | Coupling | Observability | Notes / evidence |
|---|---|---|---|---|---|---|
| `internal/recall` (+eval) | A | B | A | B | A | golden-set eval gate wired into `make check` (#38/#22); usage-heat phased (P1.3) |
| `internal/worker/*` (distill) | A | B | B | B | B | consolidation gates (P2.1 #49); extract/scene/memory layered; modern-Go idiom sweep (PR #69): errors.Is/range-int/wg.Go |
| `internal/httpserver` | B | C | B | B | B | doctor metrics endpoints; TestDoctorMetricsEndpoint was flaky (#59, fixed); sinceUntil fixture now date-derived (#71 — the hardcoded 2026-08-02 rotted and reddened main) |
| `internal/inspect` | B | B | B | A | B | pagination + prefix filters (#37) |
| `internal/hygiene` | B | B | B | A | B | doctor report + reconsolidate (P3.4 #51) |
| `internal/appshell` | B | B | B | A | C | tray + sidecar supervision; release-signed; B04 flake root-caused to the test's fixed 5s log-poll window (#70, 2026-09-06) — deterministic delay-injection regression (`RMB_TEST_DAEMON_WRITE_DELAY`, test-only) on `fix/B04-flaky-daemon-log-poll`; `*os.File` guard intact; tolerant detection pending phase 3, then re-grade Tests; bootstrap test expectations now GOOS-conditional (#68, caught by first Linux CI run) |
| `internal/config` | C | B | C | B | C | **secrets still plaintext in config.yaml — P3.5 open (F06)** |
| `cmd/*` | B | A | B | A | B | thin entrypoints |
| `webui/` | C | C | B | B | C | F12 dropped the Memories Corrections tab (add/retract stay on memory detail). Mock selftest 49 asserts (F10); `make check` runs `webui-verify` (F11 #60); still no component/e2e tests; SettingsPage split is F05 |
| `internal/db` | A | B | B | A | B | goose SQL migrations embedded; upgrade-scenario tests (00008) |
| harness (Makefile/scripts/.harness) | A | A | A | A | A | F08 bug workflow + F09 PR CI + branch protection + **F11 webui-verify in `make check` (#60)**; three merge rules mechanically enforced; CI caught 4 test bugs in its first 3 runs (#68/#71/#72 + hermetic fixtures) |
| `internal/update` | B | B | B | A | B | manifest fixtures cover the running platform (#68-era fix in PR #67); B03/#15 passing (PR #77) |
| `internal/hook`, `internal/setup` (agent integrations) | A | B | B | A | B | cursor/cc/codex/opencode/pi/workbuddy/zcode; ZCode payload shape verified against the installed client bundle (#61 fix, 2026-08-29): Stop carries no user prompt → UserPromptSubmit capture hook + sidecar pairing; #62 session-key normalization mirrors the opencode precedent with total-function tests; hook tests modernized in PR #69 |

## Reading the table

- **C in webui tests**: lib-level contract selftest (F10 L2) plus compile/lint gate in `make check` (F11); still no component/e2e tests — UI changes lean on manual smoke after `make check`.
- **C in config observability**: no validation warnings surfaced to users on bad config.
- Twice-in-a-row C/D on the same dimension ⇒ promote a fix feature in `feature_list.json`.
