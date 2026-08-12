import type { ConfigView } from "./types";

/** Matches internal/config/config.go Default() pipeline values. */
export const DEFAULT_PIPELINE: ConfigView["pipeline"] = {
  l1_poll_interval: "15s",
  l2_poll_interval: "15s",
  l3_poll_interval: "300s",
  embed_poll_interval: "30s",
  l1_every_n: 8,
  l1_idle_seconds: "600s",
  l1_warmup: true,
  l2_delay_after_l1: "90s",
  l1_max_turns_per_batch: 8,
  l1_max_chars_per_batch: 24000,
  l2_max_atoms_per_batch: 60,
  l3_max_atoms_per_batch: 60,
  embed_batch_size: 32,
  l1_min_concurrency: 1,
  l1_max_concurrency: 8,
  l2_min_concurrency: 1,
  l2_max_concurrency: 4,
};
