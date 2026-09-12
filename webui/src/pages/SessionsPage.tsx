import { useEffect, useState } from "react";
import { DEFAULT_PAGE_SIZE, Pagination } from "../components/Pagination";
import { EmptyState, ErrorNote, ListSkeleton } from "../components/EmptyState";
import { SessionListRow } from "../components/SessionListRow";
import { pageSessions } from "../lib/api";
import type { SessionRow } from "../lib/types";
import { sessionDateGroupLabel } from "../lib/format";
import { useI18n } from "../i18n";

export function SessionsPage() {
  const { t, lang } = useI18n();
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

  if (loading && !rows.length) return <ListSkeleton rows={6} />;
  if (error) return <ErrorNote>{error}</ErrorNote>;

  const locale = lang === "zh" ? "zh-CN" : "en-US";
  const groups: { label: string; rows: SessionRow[] }[] = [];
  for (const row of rows) {
    const label = sessionDateGroupLabel(
      row.last_turn_at ?? row.updated_at,
      t.sessions.today,
      t.sessions.yesterday,
      locale,
    );
    const last = groups[groups.length - 1];
    if (last && last.label === label) last.rows.push(row);
    else groups.push({ label, rows: [row] });
  }

  return (
    <div className="space-y-4">
      <p className="text-xs text-rmb-muted">
        {total} {t.sessions.conversations}
      </p>

      {rows.length === 0 ? (
        <EmptyState title={t.sessions.empty} action={{ to: "/agents", label: t.agents.setUp }} />
      ) : (
        <div className="rounded-md border border-rmb-line">
          {groups.map((group, gi) => (
            <section key={group.label} className={gi > 0 ? "border-t border-rmb-line" : ""}>
              <h2 className="bg-rmb-fill/70 px-3 py-1.5 text-xs font-medium text-rmb-muted">
                {group.label}
              </h2>
              <div className="divide-y divide-rmb-line">
                {group.rows.map((row) => (
                  <SessionListRow key={row.id} row={row} timeOnly />
                ))}
              </div>
            </section>
          ))}
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
      )}
    </div>
  );
}
