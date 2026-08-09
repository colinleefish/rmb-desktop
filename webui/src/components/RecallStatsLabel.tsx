import type { RecallStats } from "../lib/types";

function count(stats: RecallStats | null | undefined, key: keyof RecallStats): number {
  const value = stats?.[key];
  return typeof value === "number" ? value : 0;
}

export function RecallStatsLabel({
  stats,
  title = "Search / cat / meta",
}: {
  stats?: RecallStats | null;
  title?: string;
}) {
  const search = count(stats, "search_count");
  const cat = count(stats, "cat_count");
  const meta = count(stats, "meta_count");
  return (
    <span className="font-mono text-xs text-rmb-gray" title={title}>
      {search} / {cat} / {meta}
    </span>
  );
}
