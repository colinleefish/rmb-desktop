import type { RecallStats } from "./types";

function count(stats: RecallStats | null | undefined, key: keyof RecallStats): number {
  const value = stats?.[key];
  return typeof value === "number" ? value : 0;
}

/** Sum of rmb CLI recall operations (search + cat + meta) on one object. */
export function recallTotal(stats: RecallStats | null | undefined): number {
  return count(stats, "search_count") + count(stats, "cat_count") + count(stats, "meta_count");
}

/** "search 7 · cat 2 · meta 1" for tooltips. */
export function recallBreakdown(stats: RecallStats | null | undefined): string {
  return `search ${count(stats, "search_count")} · cat ${count(stats, "cat_count")} · meta ${count(stats, "meta_count")}`;
}
