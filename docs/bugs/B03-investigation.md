# Bug Investigation — B03: tray repeats already-installed update

- **Bug ID / Issue**: B03 / #15
- **Investigator**: cursor agent session
- **Date**: 2026-09-12

## TL;DR — root cause in one line

`checkForUpdate` passes `version.Version` (shell) into `update.Check`, so a sidecar-only update never advances the comparison baseline.

## Evidence chain

1. **Tray re-offers v0.2.7 after successful sidecar install**
   ```
   $ gh issue view 15 --repo colinleefish/rmb-desktop
   ```
   → Shell stayed at 0.2.6; `rmb version` and `/api/v1/version` report 0.2.7 after tray install; "Check for Updates" still offers 0.2.7.

2. **Update check baseline is the shell binary**
   ```
   $ rg 'update.Check' internal/appshell/updater.go
   return update.Check(ctx, updateFeeds(), version.Version)
   ```
   → `internal/appshell/updater.go:56` uses compile-time shell version.

3. **Apply only swaps sidecars under ~/.rmb/bin**
   ```
   $ rg 'func Apply' internal/update/apply.go
   ```
   → `internal/update/apply.go:80` replaces `rmb` / `rmbd-desktop`; no stamp or shell relaunch.

## Root cause

- **Defect location**: `internal/appshell/updater.go:55-56`
- **What is wrong**: `checkForUpdate` compares the release feed against `version.Version` from the menu-bar shell, not the version of binaries in `~/.rmb/bin`.
- **Why it produces the symptom**: Self-update installs newer sidecars while the shell keeps its DMG-linked version. Every poll still sees feed > shell, so the tray keeps showing "Install Update" for a version already on disk.

## Regression test(s)

| Test | File | Expected (post-fix) | Actual (pre-fix, observed) |
|---|---|---|---|
| `TestB03_updateCheckVersionUsesInstalledSidecar` | `internal/appshell/updater_b03_test.go` | baseline `99.99.99` from stamp | returns `version.Version` |

Run: `go test ./internal/appshell -run TestB03_updateCheckVersionUsesInstalledSidecar -count=1`

## Fix direction (not a patch)

- Suggested approach: `updateCheckVersion(installDir)` — stamp file written by `update.Apply`, else daemon `/api/v1/version`, else shell version; wire `checkForUpdate` to use it.
- Files to touch: `internal/appshell/updater.go`, `internal/update/apply.go`, `internal/update/installed.go`
- Risks / edge cases: installs that updated before stamp exists rely on daemon fallback; stamp write failure should not roll back a successful swap.

## Out of scope (→ new bug entries)

- None.

## Exit checklist

- [x] Regression test fails on unfixed code for the right reason (not a compile error)
- [x] `regression {file, cmd}` recorded in `bug_list.json`
- [x] This doc committed on `fix/B03-tray-update-baseline`
- [ ] Summary comment posted on the issue
- [ ] `make bug-state B=B03 S=diagnosed` passed (gate re-runs the failing test)
