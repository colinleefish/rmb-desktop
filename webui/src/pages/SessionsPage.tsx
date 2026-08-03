import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { DEFAULT_PAGE_SIZE, Pagination } from "../components/Pagination";
import { pageSessions } from "../lib/api";
import type { SessionRow } from "../lib/types";
import { sessionSourceLabel, sessionSourceShortLabel, formatDateTime } from "../lib/format";
import { useI18n } from "../i18n";

function SourceBadge({ source }: { source?: string | null }) {
  const label = sessionSourceLabel(source);
  if (label === "—") {
    return <span className="text-rmb-gray">—</span>;
  }
  const short = sessionSourceShortLabel(source);
  return (
    <span
      className="inline-block max-w-full truncate rounded bg-rmb-light px-1 py-0.5 text-[10px] font-medium text-rmb-dark"
      title={label}
    >
      {short}
    </span>
  );
}

function pipelineLedClass(status?: string): string {
  switch ((status ?? "").toLowerCase()) {
    case "running":
      return "bg-rmb-accent ring-1 ring-rmb-accent rmb-led-running animate-pulse";
    case "pending":
      return "bg-rmb-accent/45 ring-1 ring-rmb-accent/60 rmb-led-pending";
    case "failed":
      return "bg-rmb-gray/70 ring-1 ring-rmb-dark/40";
    case "finished":
      return "bg-rmb-dark/75 ring-1 ring-rmb-gray/50";
    case "idle":
    default:
      return "bg-rmb-gray/20 ring-1 ring-rmb-gray/25";
  }
}

function PipelineLights({
  t1,
  t2,
  t3,
}: {
  t1?: string;
  t2?: string;
  t3?: string;
}) {
  const tiers = [
    { label: "T1", status: t1 },
    { label: "T2", status: t2 },
    { label: "T3", status: t3 },
  ];

  return (
    <div className="flex items-center justify-end gap-2">
      {tiers.map(({ label, status }) => (
        <span
          key={label}
          className="inline-flex items-center gap-1"
          title={`${label}: ${status ?? "—"}`}
        >
          <span className="text-[10px] font-medium text-rmb-gray/55">{label}</span>
          <span
            className={`size-2 shrink-0 rounded-full ${pipelineLedClass(status)}`}
            aria-hidden
          />
        </span>
      ))}
    </div>
  );
}

export function SessionsPage() {
  const { t } = useI18n();
  const [rows, setRows] = useState<SessionRow[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(DEFAULT_PAGE_SIZE);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    pageSessions({ limit, offset, sort: "updated", order: "desc" })
      .then((page) => {
        setRows(page.items);
        setTotal(page.total);
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [limit, offset]);

  if (loading && !rows.length) {
    return <p className="text-rmb-gray">{t.sessions.loading}</p>;
  }
  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold text-rmb-dark">{t.sessions.title}</h1>
        <p className="text-rmb-gray">
          {total} {t.sessions.conversations}
        </p>
      </div>
      <div className="overflow-x-auto rounded-xl border border-rmb-gray/20 bg-white">
        {rows.length === 0 ? (
          <p className="px-4 py-8 text-center text-rmb-gray">{t.sessions.empty}</p>
        ) : (
          <table className="w-full table-fixed text-left text-sm">
            <colgroup>
              <col className="w-[27%]" />
              <col className="w-[24%]" />
              <col className="w-24" />
              <col className="w-12" />
              <col className="w-32" />
              <col className="w-36" />
            </colgroup>
            <thead className="border-b border-rmb-gray/15 bg-rmb-light text-rmb-gray">
              <tr>
                <th className="px-4 py-3 font-medium">{t.sessions.colUid}</th>
                <th className="px-4 py-3 font-medium">{t.sessions.colAbstract}</th>
                <th className="px-4 py-3 text-right font-medium">{t.sessions.colTurns}</th>
                <th className="px-4 py-3 font-medium">{t.sessions.colSource}</th>
                <th className="px-4 py-3 text-right font-medium">{t.sessions.colPipeline}</th>
                <th className="px-4 py-3 font-medium">{t.sessions.colUpdated}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.id}
                  className="h-[4.5rem] border-b border-rmb-gray/10 last:border-0"
                >
                  <td className="px-4 align-middle">
                    <Link
                      to={`/sessions/${encodeURIComponent(row.session_key)}`}
                      className="block truncate font-mono text-sm font-medium text-rmb-dark hover:text-rmb-accent hover:underline"
                      title={row.session_key}
                    >
                      {row.session_key}
                    </Link>
                  </td>
                  <td className="px-4 align-middle">
                    <div className="flex h-10 items-center">
                      <p
                        className="line-clamp-2 text-sm leading-5 text-rmb-gray"
                        title={row.abstract ?? undefined}
                      >
                        {row.abstract ?? "—"}
                      </p>
                    </div>
                  </td>
                  <td className="px-4 align-middle text-right tabular-nums text-rmb-dark">
                    {row.turn_count}
                    {t.sessions.turnUnit}
                  </td>
                  <td className="px-4 align-middle">
                    <SourceBadge source={row.source} />
                  </td>
                  <td className="px-4 align-middle">
                    <PipelineLights
                      t1={row.t1_status}
                      t2={row.t2_status}
                      t3={row.t3_status}
                    />
                  </td>
                  <td className="px-4 align-middle text-xs text-rmb-gray">
                    {formatDateTime(row.last_turn_at ?? row.updated_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <Pagination
          total={total}
          limit={limit}
          offset={offset}
          onPageChange={setOffset}
          onLimitChange={(next) => {
            setLimit(next);
            setOffset(0);
          }}
        />
      </div>
    </div>
  );
}
