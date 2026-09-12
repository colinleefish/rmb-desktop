import type { RecallStats } from "../lib/types";
import { recallBreakdown, recallTotal } from "../lib/recallStats";

// The total is the sum of rmb CLI recall operations on this object
// (search / cat / meta). The breakdown lives in the tooltip.
export function RecallStatsLabel({
  stats,
  unit,
  className = "",
  /** Same weight/color as surrounding meta text (profile footer, modal). */
  uniform = false,
}: {
  stats?: RecallStats | null;
  unit?: string;
  className?: string;
  uniform?: boolean;
}) {
  const total = recallTotal(stats);
  return (
    <span
      className={`inline-flex items-center gap-1 text-xs text-rmb-muted ${className}`}
      title={recallBreakdown(stats)}
    >
      <span className={uniform ? "tabular-nums" : "font-medium tabular-nums text-rmb-dark"}>
        {total}
      </span>
      {unit ? <span>{unit}</span> : null}
    </span>
  );
}
