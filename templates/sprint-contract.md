# Sprint Contract — F<id>: <name>

> Fill this BEFORE starting implementation. One feature = one contract = one session (L07/L10).
> Negotiate scope with the user; record it here; drift means stop-and-renegotiate, not silently expand.

- **Feature ID**: F<id> (feature_list.json)
- **Issue**: #<n>
- **Branch / worktree**: `<prefix>/<slug>` / `../rmb-desktop-<slug>`
- **Date**: YYYY-MM-DD

## Scope

**In** (this contract delivers):
- ...

**Out** (explicitly excluded — do not touch):
- ...

## Definition of Done

The feature's `layers` in `feature_list.json` (L1 static / L2 runtime / L3 system) all pass via
`make verify-feature F=<id>`. No hand-waved completion: state moves to `passing` only with evidence.

## Risks / open questions

- ...

## Timebox

- Planned: <duration>. If exceeded: checkpoint in PROGRESS.md, stop, renegotiate.
