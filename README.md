# RMB Desktop

> v0.2.7 is a release-pipeline smoke-test build (no functional changes).

English / [简体中文](README_CN.md)

**Give your AI coding agents a memory that lasts.**

RMB remembers what you told Cursor, Claude Code, Codex, and other agents — so they stop asking the same questions every session. Everything runs on your Mac (or PC). Your data stays on your machine.

Website: [re-mem-ber.me](https://re-mem-ber.me)

## What it does

AI agents forget everything when a chat ends. RMB fixes that in three steps:

1. **Capture** — quietly records conversations from your coding agents
2. **Remember** — turns those chats into useful facts in the background
3. **Recall** — lets agents search past knowledge before asking you again

Think of it as a shared notebook for all your agents.

## Who it's for

Anyone who uses AI coding tools and is tired of re-explaining project context, preferences, or decisions.

Supports: 

- [x] Cursor
- [x] Claude Code
- [x] Codex
- [x] OpenCode
- [x] Pi

## Download and Setup

1. Download from [re-mem-ber.me](https://re-mem-ber.me) or [GitHub Releases](https://github.com/colinleefish/rmb-desktop/releases)
2. Open the app — you'll see an RMB icon in the menu bar
3. Follow the setup wizard (pick your agents + API keys)
4. Keep coding — RMB runs in the background


### First launch on macOS

Releases are signed and notarized (Developer ID), so macOS opens them normally after the usual first-launch confirmation. If you installed an early unsigned build and macOS calls it "damaged", run this in Terminal and open it again:

```bash
xattr -dr com.apple.quarantine "/Applications/RMB Desktop.app"
```

## Privacy

- Runs entirely on your device — no cloud account required
- Needs an LLM API key only for turning chats into memories
- Optional multi-device sync may come later; v1 is single-device

## License

TBD
