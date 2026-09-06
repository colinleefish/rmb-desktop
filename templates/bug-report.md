# Bug Report — B<nn>: <one-line symptom>

> **Phase 1 artifact** (AGENTS.md "Bug workflow"). Fill this in, then:
> 1. File the issue: `bash scripts/with-proxy.sh gh issue create --title "bug(<scope>): <symptom>" --label bug --label "area/<scope>" --label "sev-<high|medium|low>" --body-file <this-file>`
>    (flaky tests use the title form `flaky: <TestName> <behavior>`)
> 2. Add the entry to `bug_list.json` (state `reported`) — shape: `bug_list.schema.json`
> Human reporters: the same form is installed at `.github/ISSUE_TEMPLATE/bug.md`, so GitHub-filed
> and agent-filed bugs arrive in one shape. The triage agent normalizes title + labels + entry.

- **Bug ID**: B<nn> (next free id in `bug_list.json`)
- **Issue**: #<n>
- **Date**: YYYY-MM-DD
- **Severity**: high | medium | low
- **Module**: `internal/<domain>` (scope↔module map in AGENTS.md)

## Symptom

One paragraph, user's perspective: what was observed. The issue title is the one-line version of this.
If a root cause is already suspected, append it parenthetically — hypotheses are allowed at phase 1.

## Reproduction steps

1. ...
2. ...

Numbered, exact commands or clicks. If not deterministic (flaky), say so and note the
failure rate you observed — the investigation agent will need statistics, not a single run.

## Environment

- App version: `<Makefile VERSION at build time / installed build>`
- OS: macOS xx.x (arm64) / ...
- Agent (capture-related bugs): claude-code / zcode / cursor / codex / opencode / pi
- Relevant config state: db path, provider settings — **never secrets**

## Expected vs Actual (user level)

- **Expected**: ...
- **Actual**: ...

## Suspected area (optional, unverified)

First guess — allowed to be wrong. The investigation agent (phase 2) proves or disproves it.
