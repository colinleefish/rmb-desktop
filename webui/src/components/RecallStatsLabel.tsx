import type { RecallStats } from "../lib/types";

function count(stats: RecallStats | null | undefined, key: keyof RecallStats): number {
  const value = stats?.[key];
  return typeof value === "number" ? value : 0;
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <span className="flex min-w-0 flex-col items-start leading-tight">
      <span className="tabular-nums">{value}</span>
      <span className="text-[10px] uppercase tracking-wide text-rmb-gray/60">{label}</span>
    </span>
  );
}

// The three numbers are per-memory counts of the rmb CLI recall operations:
// search / cat / meta (e.g. `rmb search` hits on this memory).
export function RecallStatsLabel({
  stats,
  title = "search / cat / meta — rmb CLI recall operations on this memory",
}: {
  stats?: RecallStats | null;
  title?: string;
}) {
  const search = count(stats, "search_count");
  const cat = count(stats, "cat_count");
  const meta = count(stats, "meta_count");
  return (
    <span
      className="inline-flex items-start gap-3 font-mono text-xs text-rmb-gray"
      title={title}
    >
      <Stat value={search} label="search" />
      <Stat value={cat} label="cat" />
      <Stat value={meta} label="meta" />
    </span>
  );
}
