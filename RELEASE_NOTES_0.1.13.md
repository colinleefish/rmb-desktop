## Pipeline health

- New **Pipeline** page: bottom-up distillation funnel, per-stage pending/running/failed/idle/waiting counts, and a needs-attention list.
- Replaced unused `GET /api/v1/browse/pipeline-state` with `GET /api/v1/browse/pipeline-health`.
- Funnel uses `*_advanced_at` so default idle T2/T3 rows are not counted as done (`waiting` = not reached yet).

## Faster distillation backlog

- L1/L2 process multiple sessions in parallel with AIMD back pressure (scale up on healthy backlog, cut on 429/timeouts).
- Large queues warm-start concurrency (defaults: L1 1–8, L2 1–4) via `l1_min/max_concurrency` and `l2_min/max_concurrency`.

## Packaging / platform

- Windows build/notarize helpers and Windows port cleanup for the tray daemon.
- Cross-platform config path via `dirs::data_dir`.
