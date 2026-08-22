# recall eval harness

A golden-set regression gate for the recall pipeline (issue #22). Every
ranking/pipeline change to `internal/recall`, `internal/inspect` or
`internal/agentmemory` triggers it in CI via `.github/workflows/eval.yml`; run
locally with:

    make eval

## What it measures

- **recall@5** — is at least one expected URI for a question in the top-5?
- **dup-rate in top-5** — fraction of top-5 result slots occupied by a
  near-duplicate body of an earlier slot (normalized-snippet equality /
  near-full containment).
- **recency-precision** — for recency-tagged questions, the `latest`-tagged
  memory must not be outranked by an older expected URI when the set is hit.

`cats-per-answer` is not measured: it needs scene-body depth that the fixture
does not capture.

## Golden set provenance

`golden.yaml` mixes:

- the 2026-08-22 retrieval audit's 8 golden questions plus its typo and
  cross-lingual probes (`docs/audit/2026-08-22-memory-retrieval-audit/`);
- `live-*` questions written from **raw session turns** sampled read-only
  across the ~20-day window (including the backfill period back to 2026-06-09)
  **before** inspecting which memories were distilled. Expected URIs were then
  pinned by verifying each against the distilled store.

The committed `testdata/golden_fixture.json` is a deterministic subset snapshot
(expected URIs + a stride sample of distractors) of that store.

## Determinism / accuracy tradeoff

The eval is fully offline and deterministic: it builds a scratch `rmb.db` from
the fixture and embeds everything (documents + queries) with `HashEmbed`, a
hash-based bag-of-tokens embedder (`internal/recall/eval/embedder.go`). This
makes CI reproducible and network-free, but `HashEmbed` approximates semantics
with lexical overlap, so numbers are a **conservative lower bound** versus the
production embedder. Treat the gates as a regression tripwire from a fixed
baseline, not as a measurement of absolute product quality.

For a full-fidelity local run, snapshot the live store and re-embed documents
with the production embedder (out of scope for v1).

## Regenerating the fixture

Read-only snapshot from a store copy (never the live daemon):

    go run -tags sqlite_fts5 ./cmd/rmb-eval snapshot \
        -db <path-to-copied-rmb.db> -golden internal/recall/eval/golden.yaml \
        -out internal/recall/eval/testdata/golden_fixture.json
