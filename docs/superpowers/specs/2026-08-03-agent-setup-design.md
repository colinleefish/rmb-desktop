# Agent Integration Hub — Design Spec

**Date:** 2026-08-03  
**Milestone:** M6 — Agent setup  
**Status:** Draft — pending review

## Summary

Add an **Agent Integration Hub** to the WebUI that helps users connect coding agents (Cursor, Claude Code, Codex) to rmb-desktop. Each agent gets a dedicated page with **preview-before-apply** for every file RMB touches. Users review current vs proposed content side-by-side before writing anything to disk.

Core interaction: **Review → Apply** (per artifact, not blind one-click install).

## Goals

1. New users can connect Cursor or Claude Code without reading README.
2. Users see exactly what will change before Apply.
3. Merge is idempotent and preserves unrelated config.
4. Recall instructions teach agents to use `rmb` CLI during sessions.

## Non-goals (v1)

- Windows hook support
- CC Switch auto-repair / health-check daemon (D16 deferred)
- Codex (phase 2 after Cursor + CC)
- MCP recall server
- Project-level hook install (user-level only)
- Auto-write Cursor User Rules if storage format is inaccessible (copy-only fallback)

---

## Information architecture

### Routes

| Route | Page |
|-------|------|
| `/ui/agents` | Hub — status cards per agent |
| `/ui/agents/cursor` | Cursor integration |
| `/ui/agents/claude-code` | Claude Code integration |
| `/ui/agents/codex` | Codex integration (phase 2) |

### Sidebar

New nav group **Integrations** (or **Agents**) between Memories and Settings footer.

---

## Per-agent artifacts

### Cursor

| # | Artifact | Path | Apply mode |
|---|----------|------|------------|
| 1 | Hook | `~/.cursor/hooks.json` | `write` |
| 2 | Recall | Cursor User Rules (Settings → Rules) | `write` or `copy_only`* |

\* Spike Cursor rules storage. If not file-backed, show full preview but Apply becomes Copy to clipboard + screenshot guide.

**Hook merge:** Add/update `hooks.stop[]` entry:

```json
{
  "command": "<resolved-rmb-path> hook-submit --source=cursor",
  "timeout": 15
}
```

Preserve all other hook events and non-RMB entries. Detect RMB-managed entries by command containing `rmb hook-submit` or `rmb-hook-dual`.

### Claude Code

| # | Artifact | Path | Apply mode |
|---|----------|------|------------|
| 1 | Hook | `~/.claude/settings.json` | `write` |
| 2 | Recall | `~/.claude/CLAUDE.md` | `write` |

**Decision:** Default recall file is `~/.claude/CLAUDE.md` (CC convention). No `AGENTS.md` alias in v1.

**Hook merge:** Add/update `hooks.Stop[].hooks[]` entry with `type: "command"`.

**Secrets:** Redact `env` API tokens in preview UI (`••••••`). Warn if `~/.claude/settings.local.json` exists (we only modify user-level settings).

### Recall block (managed section)

Used in Cursor User Rules and `CLAUDE.md`:

```markdown
<!-- rmb:recall:start -->
ALWAYS RUN `rmb` cli at the start of each conversation.
Search before asking the user. Follow rmb://agent for recall rules.
<!-- rmb:recall:end -->
```

Re-apply replaces content between markers (idempotent).

Content sourced from `rmb://agent` memory template; per-agent framing only.

---

## UX: ConfigDiffReview component

Reusable panel for every artifact:

```
ConfigDiffReview
├── Label + file path (monospace)
├── Status badge: Not found | Unchanged | Will modify | Will create
├── View toggle: Side-by-side (JSON) | Unified diff (markdown)
├── Left pane:  "Current"
├── Right pane: "After apply"
├── Summary: +N added, ~N changed, 0 removed
├── Warning callouts (secrets redacted, local overrides, etc.)
└── Actions: [Cancel] [Apply] or [Copy to clipboard]
```

**Apply is per artifact.** User can install hook first, recall instructions later.

### Agent page layout (vertical checklist)

```
§1 Hook capture
   Explanation → ConfigDiffReview → [Apply hook]

§2 Recall instructions
   Explanation → ConfigDiffReview → [Apply] or [Copy]

§3 Verify
   "Chat in agent → check Sessions" + link to /ui/sessions?source=<agent>
```

---

## API

### Endpoints

```
GET  /api/v1/setup/status
GET  /api/v1/setup/:agent/preview
POST /api/v1/setup/:agent/apply
```

Agents: `cursor`, `claude-code`, `codex` (later).

### Preview response

```json
{
  "agent": "cursor",
  "artifacts": [
    {
      "id": "hooks",
      "path": "~/.cursor/hooks.json",
      "exists": true,
      "current": "{ ... }",
      "proposed": "{ ... }",
      "change_type": "modify",
      "summary": "Add stop hook for rmb hook-submit",
      "apply_mode": "write",
      "warnings": []
    },
    {
      "id": "user_rules",
      "path": "Cursor Settings → Rules",
      "exists": true,
      "current": "...",
      "proposed": "...",
      "change_type": "append",
      "summary": "Append RMB recall block",
      "apply_mode": "copy_only",
      "warnings": ["Rules stored in Cursor internal storage"]
    }
  ]
}
```

`change_type`: `create` | `modify` | `append` | `unchanged`  
`apply_mode`: `write` | `copy_only`

### Apply request

```json
{
  "artifacts": ["hooks"]
}
```

- Creates `.<file>.rmb.bak` before write
- Returns updated preview for applied artifacts
- Idempotent: second apply → `unchanged`

### CLI (power users)

```
rmb setup --agent=cursor|cc [--dry-run]
rmb setup status [--agent=<name>] [--json]
```

`--dry-run` prints preview JSON (same as API preview).

---

## Backend: `internal/setup`

```
internal/setup/
├── agent.go       # registry: agent → artifacts, paths
├── cursor.go      # hooks.json merge + user rules block
├── claude.go      # settings.json merge + CLAUDE.md block
├── preview.go     # Preview(agent) → artifacts[]
├── apply.go       # Apply(agent, artifactIDs) with backup
├── detect.go      # IsRMBManaged(), AlreadyConfigured()
└── redact.go      # strip secrets from settings.json preview
```

### Merge rules

1. Never delete unrelated keys or hooks.
2. Update RMB-managed entries in place.
3. Pretty-print JSON on write (2-space indent).
4. Malformed JSON → preview error, block apply.
5. Backup before every write.

### Tests (table-driven)

- Empty file → create
- Unrelated hooks preserved
- Existing RMB hook → update in place
- Malformed JSON → error
- CLAUDE.md with existing RMB block → replace markers only
- settings.json env keys redacted in preview output

---

## WebUI files

```
webui/src/
├── pages/agents/
│   ├── AgentsHubPage.tsx
│   ├── CursorAgentPage.tsx
│   └── ClaudeCodeAgentPage.tsx
├── components/agents/
│   ├── ConfigDiffReview.tsx
│   ├── ArtifactSection.tsx
│   └── AgentStatusCard.tsx
└── lib/setupApi.ts
```

Follow existing patterns: `SettingsPage` tabs, `Sidebar` nav groups, i18n EN+ZH.

---

## Implementation phases

| Phase | Scope | Exit criteria |
|-------|-------|---------------|
| **0** | `internal/setup` preview+apply for hooks.json, settings.json | Unit tests pass; `rmb setup --dry-run` works |
| **1** | ConfigDiffReview + Cursor hooks section | User previews and applies hooks.json from UI |
| **2** | Cursor user rules (spike storage → write or copy) | Recall block previewable; apply or copy works |
| **3** | CC settings.json + CLAUDE.md | Full CC page with preview-before-apply |
| **4** | Hub page, status API, verify flow, i18n | Overview shows agent status; sessions link works |

**Estimate:** ~2 weeks (matches M6 in `plan/implementation.md`).

---

## Success criteria

- [ ] User previews hooks.json diff before Apply on Cursor page
- [ ] User previews CLAUDE.md diff before Apply on CC page
- [ ] Re-running setup is safe (idempotent, backup exists)
- [ ] `rmb setup status --json` matches WebUI status
- [ ] Chat in Cursor/CC → session appears in `/ui/sessions` within one hook cycle
- [ ] Unrelated hooks and settings keys are never modified

---

## Open items

| Item | Owner | Notes |
|------|-------|-------|
| Cursor User Rules storage spike | Phase 2 | Determines write vs copy_only |
| Codex hook parser + page | Phase 2+ | Not in v1 |
| Screenshot assets for manual fallback | Phase 4 | `webui/public/guides/<agent>/` |

---

## References

- `plan/implementation.md` — M6 milestone
- `plan/local-first-desktop.md` — D15 (hook install), D17 (agents v1)
- `internal/hook/` — existing cursor + cc parsers
- `scripts/rmb-hook-dual.sh` — dual-submit pattern (document in Advanced section)
