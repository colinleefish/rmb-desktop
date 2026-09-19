# PROGRESS.md

## Current State

**feat/intel-dmg-build (F13 passing)** — Intel MacBook DMG path: `make build-intel` cross-compiles `bin/intel/{rmb,rmbd,rmb-app}` (x86_64 cgo, minos 13.0); `make app-build-intel` assembles `dist/intel/RMB Desktop.app` + `dist/RMB Desktop_0.2.11_amd64.dmg` (ad-hoc signed, 25 MB). Verified: `file`/`lipo` x86_64, minos gate in build-macos-app.sh, `hdiutil verify` + mount check, and the amd64 binary executes under Rosetta 2. `make check` green via `make verify-feature F=F13` (all layers). Arm64 pipeline untouched.

## In Progress

- PR for `feat/intel-dmg-build` pending merge (integration session).

## Next Steps

1. Merge `feat/intel-dmg-build` → main (PR, pr-check green).
2. If Intel Macs become a supported platform: add `macos.amd64` sidecars to `build-sidecar-bundles.sh` + manifest and fold the amd64 DMG into `make release` (then notarize). Until then Intel installs don't self-update.
3. Carry-over from 0.2.11 (if not already done): confirm feed `latest.json` = 0.2.11.
4. F05 settings strangler per `plan/webui-refactor.md`.

## Blockers

None. Note: "old" Intel MacBooks still need macOS 13 Ventura+ — Go 1.27's Darwin floor; machines stuck on Monterey 12 or older cannot run these binaries.
