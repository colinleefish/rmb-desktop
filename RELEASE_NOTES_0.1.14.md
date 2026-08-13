## Stability

- SQLite write contention: `BEGIN IMMEDIATE` for worker transactions; busy timeout / connection tuning so parallel L1/L2 no longer spuriously hit `database is locked`.
- Raise default L1/L2 concurrency ceilings (64 / 16) now that DB locking is safer.

## Distillation quality

- Split oversized L2 scene prompts by scene-name groups so LLM output is less likely to truncate/parse-fail.

## Recall

- Vector recall uses sqlite-vec `vec_distance_cosine` KNN instead of loading all embeddings into Go for a linear scan.
