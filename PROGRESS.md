# PROGRESS.md

## Current State

**release/0.2.11** — pin `MACOSX_DEPLOYMENT_TARGET=13.0` so Go builds on macOS 26 no longer embed `minos 26.0` (broke Sequoia 15). VERSION → `0.2.11`.

## In Progress

- Ship `v0.2.11` (DMG + sidecars + R2 feed)

## Next Steps

1. Merge `release/0.2.11` → main; `make release VERSION=0.2.11` (+ `PUBLISH_R2=1`)
2. Confirm DMG `otool` minos ≤ 13.0 and `curl https://releases.re-mem-ber.me/latest.json` = 0.2.11
3. F05 settings strangler per `plan/webui-refactor.md`

## Blockers

None.
