import { Link } from "react-router-dom";
import type { SessionRow } from "../lib/types";
import { distillStatus, tierStatusTitle, type DistillStatus } from "../lib/sessionStatus";
import { formatDateTime, formatDateTimeMonoClass, formatTimeOfDay } from "../lib/format";
import { useI18n } from "../i18n";
import { AgentChip } from "./AgentChip";
import { StatusPill, type PillTone } from "./StatusPill";

const TONE: Record<DistillStatus, PillTone> = {
  distilled: "info",
  distilling: "ok",
  pending: "neutral",
  failed: "danger",
};

/** Grid for Overview “needs attention” rows: stage pill · body · time/action. */
export const attentionListRowGridClass =
  "grid min-h-14 min-w-0 grid-cols-[5.5rem_minmax(0,1fr)_11.5rem] items-center gap-4 px-4 py-3";

export function ListRowTextStack({
  primary,
  secondary,
  primaryMono = true,
}: {
  primary: string;
  secondary?: string | null;
  /** Session keys use mono; hints use proportional. */
  primaryMono?: boolean;
}) {
  return (
    <div className="min-w-0">
      <div
        className={`truncate font-medium text-rmb-dark ${
          primaryMono ? "font-mono text-[13px]" : "text-sm"
        }`}
        title={primary}
      >
        {primary}
      </div>
      {secondary ? (
        <div className="truncate text-xs text-rmb-muted" title={secondary}>
          {secondary}
        </div>
      ) : null}
    </div>
  );
}

export function ListRowTimestamp({ at, title }: { at: string; title?: string }) {
  return (
    <span
      className={`text-right text-xs text-rmb-faint ${formatDateTimeMonoClass}`}
      title={title}
    >
      {formatDateTime(at)}
    </span>
  );
}

export function DistillPill({
  t1,
  t2,
  t3,
}: {
  t1?: string;
  t2?: string;
  t3?: string;
}) {
  const { t } = useI18n();
  const status = distillStatus(t1, t2, t3);
  const label = {
    distilled: t.sessions.statusDistilled,
    distilling: t.sessions.statusDistilling,
    pending: t.sessions.statusPending,
    failed: t.sessions.statusFailed,
  }[status];
  return (
    <StatusPill tone={TONE[status]} title={tierStatusTitle(t1, t2, t3)}>
      {label}
    </StatusPill>
  );
}

/** One session in a feed: agent · key + abstract · status · turns · time. */
export function SessionListRow({
  row,
  timeOnly = false,
}: {
  row: SessionRow;
  /** Inside a date group the date is redundant; show only HH:MM:SS. */
  timeOnly?: boolean;
}) {
  const { t } = useI18n();
  const when = row.last_turn_at ?? row.updated_at;
  const gridCols = timeOnly
    ? "grid-cols-[7.5rem_minmax(0,1fr)_auto_4.5rem_7.5rem]"
    : "grid-cols-[7.5rem_minmax(0,1fr)_auto_4.5rem_11.5rem]";

  return (
    <Link
      to={`/sessions/${encodeURIComponent(row.session_key)}`}
      className={`grid min-h-14 ${gridCols} min-w-0 items-center gap-4 px-4 py-2 transition-colors hover:bg-rmb-fill`}
    >
      <AgentChip source={row.source} />
      <ListRowTextStack primary={row.session_key} secondary={row.abstract} />
      <DistillPill t1={row.t1_status} t2={row.t2_status} t3={row.t3_status} />
      <span className="text-right text-xs text-rmb-muted">
        {row.turn_count}
        {t.sessions.turnUnit}
      </span>
      {timeOnly ? (
        <span
          className={`text-right text-xs text-rmb-faint ${formatDateTimeMonoClass}`}
          title={formatDateTime(when)}
        >
          {formatTimeOfDay(when)}
        </span>
      ) : (
        <ListRowTimestamp at={when} />
      )}
    </Link>
  );
}
