# Quality Document — module health scores

> New sessions read this to know where to be careful. Updated at clock-out for touched modules
> (AGENTS.md Clock-out). Grades A (strong) → D (debt). Scores are honest opinions backed by
> evidence pointers, not vanity metrics.

| Module | Tests | Complexity | Docs | Coupling | Observability | Notes / evidence |
|---|---|---|---|---|---|---|
| `internal/recall` (+eval) | A | B | A | B | A | golden-set eval gate wired into `make check` (#38/#22); usage-heat phased (P1.3) |
| `internal/worker/*` (distill) | A | B | B | B | B | consolidation gates (P2.1 #49); extract/scene/memory layered; modern-Go idiom sweep (PR #69): errors.Is/range-int/wg.Go |
| `internal/httpserver` | B | C | B | B | B | doctor metrics endpoints; TestDoctorMetricsEndpoint was flaky (#59, fixed) |
| `internal/inspect` | B | B | B | A | B | pagination + prefix filters (#37) |
| `internal/hygiene` | B | B | B | A | B | doctor report + reconsolidate (P3.4 #51) |
| `internal/appshell` | B | B | B | A | C | tray + sidecar supervision; release-signed; B04 flake root-caused to the test's fixed 5s log-poll window (#70, 2026-09-06) — deterministic delay-injection regression (`RMB_TEST_DAEMON_WRITE_DELAY`, test-only) on `fix/B04-flaky-daemon-log-poll`; `*os.File` guard intact; tolerant detection pending phase 3, then re-grade Tests |
| `internal/config` | C | B | C | B | C | **secrets still plaintext in config.yaml — P3.5 open (F06)** |
| `cmd/*` | B | A | B | A | B | thin entrypoints |
| `webui/` | D | C | C | B | C | **known-weak: 604-line SettingsPage, tier-based nav, mock dead weight — refactor planned (F03–F05, plan/webui-refactor.md)** |
| `internal/db` | A | B | B | A | B | goose SQL migrations embedded; upgrade-scenario tests (00008) |
| harness (Makefile/scripts/.harness) | B | A | A | A | A | this refactor (#53–#58); audit 7/7 critical |
| `internal/hook`, `internal/setup` (agent integrations) | A | B | B | A | B | cursor/cc/codex/opencode/pi/workbuddy/zcode; ZCode payload shape verified against the installed client bundle (#61 fix, 2026-08-29): Stop carries no user prompt → UserPromptSubmit capture hook + sidecar pairing; #62 session-key normalization mirrors the opencode precedent with total-function tests; hook tests modernized in PR #69 |

## Reading the table

- **D in webui tests**: no component/e2e tests at all; any change relies on `npm run build` only.
- **C in config observability**: no validation warnings surfaced to users on bad config.
- Twice-in-a-row C/D on the same dimension ⇒ promote a fix feature in `feature_list.json`.
