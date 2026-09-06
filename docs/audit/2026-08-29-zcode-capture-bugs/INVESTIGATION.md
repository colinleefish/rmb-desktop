# Investigation — ZCode capture bugs (#61, #62)

> **Date**: 2026-08-29 · **Investigator**: agent session (phase 2 of the 3-phase
> procedure: issues → investigation → fix) · **Status**: investigation complete,
> fix NOT started
> **Affected version**: 0.2.10-dev.1 (commit 80006f0, F07 zcode integration)
> **Issues**: [#61 assistant-only turns](https://github.com/colinleefish/rmb-desktop/issues/61) ·
> [#62 sess_-prefixed session keys](https://github.com/colinleefish/rmb-desktop/issues/62)

This document is the complete context handoff for the fixing agent. Everything
below was verified against live data and the installed ZCode client — nothing
is speculation. Sections: [1 Bug 1](#1-bug-61-turns-are-assistant-only),
[2 Bug 2](#2-bug-62-session-keys-are-sess_-prefixed),
[3 test matrix](#4-test-cases-expected-vs-actual),
[4 fixer handoff](#5-handoff-for-the-fixing-agent).

---

## 1. Bug #61 — turns are assistant-only

### 1.1 Symptom (live data)

Every turn in every ZCode session in the local rmb.db has exactly one message,
role `assistant`:

```
sqlite3 "$RMB_DB" "select s.session_key, json_array_length(t.messages_json),
  (select group_concat(json_extract(m.value,'$.role'), ',') from json_each(t.messages_json) m)
  from session_turns t join sessions s on t.session_id=s.id where s.source='zcode'"
sess_a2fbea3f-…|1|assistant
sess_a6f1b071-…|1|assistant   (×5)
sess_f9c1b16f-…|1|assistant
```

Each Stop hook logged `reason=last_assistant_message only (no user found)`.

### 1.2 Root cause — three stacked facts

**Fact A — rmb assumed a Claude-Code-shaped transcript.**
`internal/hook/zcode.go` (`ParseZCodePayload`) calls `claudeFindLastUserPrompt(p.TranscriptPath)`
to recover the user text. That parser (internal/hook/claude.go) only accepts
rows of the shape `{"type":"user","message":{…}}`.

**Fact B — ZCode's Stop transcript is not that.** Verified by decompiling the
installed client (`/Applications/ZCode.app/Contents/Resources/glm/zcode.cjs`,
bundle for the running ZCode 3.10.x). The Stop hook runner:

```js
// PUr — Stop event context; NOTE: no prompt field
this.hookRunner.run({ agentName, cwd, hookEventName:on.Stop, mode,
  responsePreview:i, responseText:e, sessionId, stopHookActive:o,
  timestamp, toolCallCount:t, traceId, turnId }, {signal:n})
```

`createClaudeCompatibleHookStdin` (s0t) spreads that context, adds snake_case
aliases, and writes a temp transcript via `formatClaudeTranscript` (Hei):

```js
function Hei(e){
  return e.hookEventName===on.Stop
    ? JDr("assistant", e.responseText ?? e.responsePreview)   // ← assistant ONLY
    : e.hookEventName===on.UserPromptSubmit
      ? JDr("user", e.prompt) : ""
}
function JDr(e,t){
  return JSON.stringify({message:{content:[{text:t,type:"text"}],role:e}}) + "\n"
}
```

So the temp `transcript.jsonl` the Stop hook receives contains **exactly one
line — the assistant message — with no row `type` field and never a user
message**. The temp dir (`$TMPDIR/zcode-claude-hook-*/`) is deleted right after
the hook exits.

**Fact C — the Stop stdin payload also has no user prompt.** Its fields are:
`session_id, hook_event_name, permission_mode, transcript_path,
last_assistant_message, stop_hook_active` (snake aliases) plus the camelCase
spread (`responseText, responsePreview, stopHookActive, toolCallCount, turnId,
traceId, timestamp, agentName, mode, cwd`). There is nothing to parse the user
text out of.

**Conclusion**: `claudeFindLastUserPrompt` can never return non-empty for a
ZCode Stop hook, so every upload degrades to assistant-only by design of the
current code. This is the materialization of the risk flagged in
`PROGRESS.md` at F07 time ("payload shape inferred from docs, not a captured
live payload").

### 1.3 Deterministic reproduction (verified)

Artifacts reproduced the bug against a recording stub HTTP server
(no real data touched):

1. Temp transcript = `{"message":{"content":[{"text":"As the company side, the 9:30-20:00 schedule nets exactly 8 hours.","type":"text"}],"role":"assistant"}}` + `\n`
2. Stop stdin = full alias payload (see 1.2 Fact C), `session_id=a6f1b071-e073-411c-81d4-04f787791151`
3. `rmb hook-submit --source=zcode --url=http://127.0.0.1:19099`

Result:

```
rmb hook-submit source=zcode action=upload reason=last_assistant_message only (no user found)
→ POST /api/v1/sessions/a6f1b071-e073-411c-81d4-04f787791151/upload
  {"source":"zcode","messages":[{"role":"assistant","content":"As the company side, …"}]}
```

A temporary in-repo test (run once, then deleted) confirmed the parser
behavior on both formats:

```
zcode-format transcript  -> user text = ""
claude-format transcript -> user text = "user question"
```

### 1.4 The pairing source ZCode offers

The **UserPromptSubmit** hook context DOES carry the prompt:

```js
// RUr — UserPromptSubmit event context
this.hookRunner.run({ agentName, attachmentsSummary, cwd,
  hookEventName:on.UserPromptSubmit, mode, prompt:e,
  sessionId:this.sessionId, timestamp, traceId, turnId }, {signal:n})
```

After `s0t` aliasing, its stdin contains `prompt` and `session_id` (and its own
temp transcript holds the user line, but stdin `prompt` is sufficient).
ZCode config-file hooks support `UserPromptSubmit` as one of the seven events,
and configuration-file hooks are confirmed working on this install (the Stop
hook has been firing since F07 was applied — hooks.enabled=true is in
`~/.zcode/cli/config.json`).

---

## 2. Bug #62 — session keys are `sess_`-prefixed

### 2.1 Symptom (live data)

```
source |session_key
zcode  |sess_f9c1b16f-b6d1-4bd3-adb0-d0212af43158
cursor |1682106d-8aae-404d-aca6-e6581a52b1fb
pi     |01a0485f-e20b-7fbc-a2e3-36abe88ce1e8
```

All non-zcode sources store bare UUIDs; all 3 zcode sessions store
`sess_<uuid>` verbatim (lowercased). Surfaced in the webui UID column and URLs
(`/ui/sessions/sess_…`).

### 2.2 Origin

`internal/hook/zcode.go`: `sessionKey = strings.ToLower(strings.TrimSpace(p.SessionID))`
— the payload's `session_id` is ZCode's native id. ZCode's own db
(`~/.zcode/cli/db/db.sqlite`, table `session`) shows ids `sess_<uuid>` for
interactive sessions and `sess_subagent_agent_<uuid>` for subagent children
(`task_type` ∈ {interactive, subagent_child}).

### 2.3 In-repo precedent (important for the fix decision)

OpenCode had the same problem and already normalizes:

```go
// internal/hook/opencode.go
func opencodeRMBSessionID(raw string) (string, error) {
    if parsed, err := uuid.Parse(sessionID); err == nil {
        return strings.ToLower(parsed.String()), nil      // canonical UUID
    }
    derived := uuid.NewSHA1(opencodeSessionNamespace, []byte("opencode:"+sessionID))
    return strings.ToLower(derived.String()), nil          // deterministic uuid5
}
```

ZCode's parser does no normalization. Note `sess_<uuid>` → simple prefix strip
yields a valid UUID, but `sess_subagent_agent_<uuid>` does not — a strip-only
approach needs a UUID-validity check + fallback (e.g. the opencode uuid5
derive) to stay total.

### 2.4 Blast radius check

- No rmb code parses session keys as UUIDs (grep for `uuid.Parse` on keys hits
  only opencode's own normalizer). The key is treated as opaque everywhere →
  this is a **consistency/display + migration-decision** bug, not breakage.
- 3 existing rows already store `sess_…` keys. A normalization change must
  decide: normalize at rest (needs a small migration/backfill) or at display
  time only. Changing derivation **without** migration would orphan existing
  sessions (new uploads would land under new keys).

---

## 4. Test cases (expected vs actual)

| ID | Setup | Action | Expected | Actual (2026-08-29) | Status |
|---|---|---|---|---|---|
| TC-1 | Realistic ZCode Stop stdin + assistant-only temp transcript (§1.3) | `rmb hook-submit --source=zcode` | 2 messages uploaded: `user` = the prompt the user typed, `assistant` = last reply | 1 message, `assistant` only; reason `last_assistant_message only (no user found)` | **FAIL → #61** |
| TC-2 | `claudeFindLastUserPrompt` on ZCode's transcript line (`{"message":{…,"role":"assistant"}}`, no row type) | parse | user text found (or, post-fix, pairing via another channel) | `""` — parser requires `"type":"user"` rows | **FAIL → #61** |
| TC-3 | ZCode Stop stdin with `last_assistant_message` present | `IsZCodePayload` | true | true | pass |
| TC-4 | ZCode Stop stdin w/o snake alias `last_assistant_message` (camel `responseText` only) | submit | (design decision for fixer: accept camel alias or reject) | rejected as "not a zcode payload" / empty-assistant skip | open question |
| TC-5 | Upload from ZCode interactive session | stored session_key | same key format as other agents (bare UUID or documented exception) | `sess_<uuid>` verbatim | **FAIL → #62** |
| TC-6 | ZCode subagent child id `sess_subagent_agent_<uuid>` | key derivation | total function: valid UUID out for ANY id (see opencode uuid5 fallback) | `sess_…` verbatim; prefix-strip alone would produce an invalid UUID | **FAIL → #62** |
| TC-7 | Existing rows with `sess_…` keys after a normalization fix | migration/backfill | old sessions remain reachable & merged with new uploads (no orphans) | n/a — not yet implemented | to define |

---

## 5. Handoff for the fixing agent

### 5.1 Suggested direction (decision yours — investigate before coding)

Bug #61: ZCode cannot deliver the prompt on Stop, so a **UserPromptSubmit
capture hook** is the only in-contract pairing source. Sketch: register a
second hook in `~/.zcode/cli/config.json` →
`hooks.events.UserPromptSubmit[].hooks[] = [{type:"command", command:"<rmb> hook-capture --agent=zcode"}]`
alongside the existing Stop hook; `hook-capture` parks `{session_id, prompt}`
in a sidecar (`~/.rmb/cache/agent-prompts/zcode/<session>.json`, atomic write,
best-effort, 7-day sweep); `ParseZCodePayload` consumes the sidecar and pairs.
`zcodeHookConfigured` must then require BOTH hooks + `hooks.enabled:true`
(ZCode silently ignores config-file hooks without the flag — that part works
today and must be preserved). Setup merge (`internal/setup/zcode.go`) must
migrate stop-only installs (every install made by 0.2.10-dev.1 is stop-only).

Bug #62: mirror `opencodeRMBSessionID`: strip `sess_` when the remainder is a
valid UUID, else deterministic uuid5 fallback; decide migration for the 3
existing rows (backfill via SQL or accept re-parenting on next upload).

### 5.2 Files map

| File | Role |
|---|---|
| `internal/hook/zcode.go` | payload gate + parse (both bugs live here) |
| `internal/hook/claude.go` | `claudeFindLastUserPrompt` (TC-2 evidence; likely unused for zcode post-fix) |
| `internal/hook/submit.go` | `--source=zcode` dispatch |
| `internal/setup/zcode.go` | config merge / `zcodeHookConfigured` / preview+apply |
| `internal/setup/rmb.go` | `hookCommand`, `isRMBHookCommand` (recognizes `rmb hook-submit`; extend if a capture command is added) |
| `cmd/rmb/main.go` | CLI dispatch (`hook-submit`; add capture subcommand if needed) |
| `internal/hook/opencode.go` | `opencodeRMBSessionID` — precedent for #62 |
| `webui/src/i18n/translations.ts` | `agents.zcode.captureHint` (mentions hook mechanics) |

### 5.3 Constraints

- ZCode hook stdin/shapes are as documented in §1 — re-verified against the
  installed client; do NOT re-derive from the public doc alone (its
  Claude-compat framing is what misled F07).
- Config-file hooks need `hooks.enabled: true` (merge already forces it — keep).
- All ZCode hook entries live under `hooks.events.<Event>`, NOT `hooks.<Event>`.
- `~/.zcode/cli/config.json` currently on this machine: Stop hook present,
  no UserPromptSubmit hook, `enabled: true`.
- Repo harness: `make check` gate, feature F07 already `passing` — a fix
  branch + worktree per `plan/parallel-work-and-versioning.md`, bug-fix tests
  added alongside.

### 5.4 Paused prior art

A direct-fix attempt was paused before completion (superseded by this
procedure). Its uncommitted work is stashed on branch
`fix/zcode-user-prompt-capture` (stash message: "WIP zcode two-hook fix
(paused — replaced by issue-driven 3-phase procedure)"; branch has no commits).
It already contains: `hook-capture` CLI subcommand, sidecar capture/parse in
`internal/hook/zcode.go`, two-hook merge + migration test in
`internal/setup/zcode.go`, updated tests. The fixing agent may `git stash pop`
on that branch, rebase onto main, and finish it — or start fresh; the test
matrix above is the acceptance bar either way.

---

## 6. Fix-phase decisions (bug #61 fix branch)

Recorded by the fixing agent on `fix/zcode-61-user-prompt-capture` (the stash
was popped and finished; see §5.4).

- **TC-4 (camel-only payload): REJECT.** `IsZCodePayload` keeps gating on the
  snake-case `last_assistant_message` (or `stop_hook_active`). ZCode always
  sends both spellings (§1.2 Fact C), so a camel-only payload would mean a
  client contract change — the turn then skips loudly ("not a zcode payload")
  instead of silently misparsing. Test:
  `TestIsZCodePayload/camel_responseText_only…` in `internal/hook/zcode_test.go`.
- **Prompt pairing channel:** sidecar file
  `~/.rmb/cache/agent-prompts/zcode/<lower(session_id)>.json`, written by
  `rmb hook-capture --agent=zcode` (UserPromptSubmit hook, atomic write,
  last-write-wins, 7-day stale sweep, silent skip on incomplete/unsafe
  payloads); consumed then deleted by `ParseZCodePayload` (Stop hook). Absent
  sidecar ⇒ assistant-only, i.e. pre-fix behavior preserved as graceful
  degradation. Session-key derivation untouched here — that is bug #62,
  owned by the parallel fix branch.
