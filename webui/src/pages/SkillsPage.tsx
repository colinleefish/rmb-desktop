import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Search } from "lucide-react";
import { DEFAULT_PAGE_SIZE, Pagination } from "../components/Pagination";
import { RecallStatsLabel } from "../components/RecallStatsLabel";
import { EmptyState, ErrorNote, ListSkeleton } from "../components/EmptyState";
import { StatusPill } from "../components/StatusPill";
import { pageSkills } from "../lib/api";
import { formatDateTime, formatDateTimeMonoClass } from "../lib/format";
import type { SkillRow } from "../lib/types";
import { useI18n } from "../i18n";

export function SkillsPage({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const [rows, setRows] = useState<SkillRow[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(DEFAULT_PAGE_SIZE);
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    pageSkills({ limit, offset, q: query || undefined, sort: "updated", order: "desc" })
      .then((page) => {
        setRows(page.items);
        setTotal(page.total);
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [limit, offset, query]);

  if (loading && !rows.length) return <ListSkeleton rows={5} />;
  if (error) return <ErrorNote>{error}</ErrorNote>;

  return (
    <div className="space-y-3">
      {!embedded ? (
        <div>
          <h1 className="text-lg font-semibold text-rmb-dark">{t.skills.title}</h1>
          <p className="text-sm text-rmb-muted">{t.skills.subtitle}</p>
        </div>
      ) : null}

      <div className="flex items-center justify-between gap-4">
        <div className="relative w-72">
          <Search
            className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-rmb-faint"
            strokeWidth={1.75}
          />
          <input
            type="search"
            value={query}
            onChange={(e) => {
              setOffset(0);
              setQuery(e.target.value);
            }}
            placeholder={t.skills.searchPlaceholder}
            className="h-8 w-full rounded-md border border-rmb-line-strong bg-white pl-8 pr-3 text-sm text-rmb-dark outline-none transition-colors placeholder:text-rmb-faint focus:border-rmb-accent"
          />
        </div>
        <span className="text-xs text-rmb-muted">
          {total} {t.skills.title.toLowerCase()}
        </span>
      </div>

      {rows.length === 0 ? (
        <EmptyState title={t.skills.empty} />
      ) : (
        <div className="rounded-md border border-rmb-line">
          <div className="divide-y divide-rmb-line">
            {rows.map((row) => (
              <Link
                key={row.slug}
                to={`/agents/skills/${encodeURIComponent(row.slug)}`}
                className="grid min-h-14 grid-cols-[minmax(0,1fr)_3rem_5rem_11.5rem] items-center gap-4 px-3 py-2 transition-colors hover:bg-rmb-fill"
              >
                <div className="min-w-0">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="truncate text-sm font-medium text-rmb-dark">{row.name}</span>
                    {(row.tags ?? []).map((tag) => (
                      <StatusPill key={tag} tone="neutral">
                        {tag}
                      </StatusPill>
                    ))}
                    <span className="hidden truncate font-mono text-[11px] text-rmb-faint lg:inline">
                      {row.uri}
                    </span>
                  </div>
                  <p className="truncate text-xs text-rmb-muted" title={row.description}>
                    {row.description}
                  </p>
                </div>
                <span className="text-right font-mono text-xs text-rmb-muted">v{row.version}</span>
                <RecallStatsLabel
                  stats={row.recall_stats}
                  unit={t.memories.recalls}
                  className="justify-end"
                />
                <span className={`text-right text-xs text-rmb-faint ${formatDateTimeMonoClass}`}>
                  {formatDateTime(row.updated_at)}
                </span>
              </Link>
            ))}
          </div>
          <Pagination
            total={total}
            offset={offset}
            limit={limit}
            onPageChange={setOffset}
            onLimitChange={setLimit}
          />
        </div>
      )}
    </div>
  );
}
