import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Wand2 } from "lucide-react";
import { DEFAULT_PAGE_SIZE, Pagination } from "../components/Pagination";
import { pageSkills } from "../lib/api";
import { formatDateTime } from "../lib/format";
import type { SkillRow } from "../lib/types";
import { useI18n } from "../i18n";

const TAG_CLASS: Record<string, string> = {
  work: "border-emerald-600/30 bg-emerald-500/10 text-emerald-700",
  personal: "border-violet-600/30 bg-violet-500/10 text-violet-700",
};

function SkillTag({ tag }: { tag: string }) {
  const key = tag.toLowerCase();
  return (
    <span
      className={[
        "rounded border px-1.5 py-0.5 text-[10px] font-medium",
        TAG_CLASS[key] ?? "border-rmb-gray/25 bg-rmb-light text-rmb-gray",
      ].join(" ")}
    >
      {tag}
    </span>
  );
}

export function SkillsPage() {
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

  if (loading && !rows.length) {
    return <p className="text-rmb-gray">{t.skills.loading}</p>;
  }
  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold text-rmb-dark">{t.skills.title}</h1>
        <p className="text-rmb-gray">{t.skills.subtitle}</p>
      </div>

      <div className="flex gap-2">
        <input
          type="search"
          value={query}
          onChange={(e) => {
            setOffset(0);
            setQuery(e.target.value);
          }}
          placeholder={t.skills.searchPlaceholder}
          className="w-full max-w-md rounded-lg border border-rmb-gray/20 bg-white px-3 py-2 text-sm text-rmb-dark"
        />
      </div>

      <div className="overflow-x-auto rounded-xl border border-rmb-gray/20 bg-white">
        {rows.length === 0 ? (
          <p className="px-4 py-8 text-center text-rmb-gray">{t.skills.empty}</p>
        ) : (
          <table className="w-full text-left text-sm">
            <thead className="border-b border-rmb-gray/15 text-rmb-gray">
              <tr>
                <th className="px-4 py-3 font-medium">{t.skills.colSkill}</th>
                <th className="px-4 py-3 font-medium">{t.skills.colRevision}</th>
                <th className="px-4 py-3 font-medium">{t.skills.colUpdated}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.slug}
                  className="group border-b border-rmb-gray/10 last:border-0 hover:bg-rmb-light/40"
                >
                  <td className="px-4 py-3">
                    <Link
                      to={`/skills/${encodeURIComponent(row.slug)}`}
                      className="flex min-w-0 items-start gap-3"
                    >
                      <span
                        className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-violet-500/10 text-violet-600"
                        aria-hidden
                      >
                        <Wand2 className="size-4" />
                      </span>
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-semibold text-rmb-dark">{row.name}</span>
                          {(row.tags ?? []).map((tag) => (
                            <SkillTag key={tag} tag={tag} />
                          ))}
                        </div>
                        <div className="font-mono text-xs text-rmb-gray">{row.uri}</div>
                        <p className="mt-1 line-clamp-2 text-sm text-rmb-gray">{row.description}</p>
                      </div>
                    </Link>
                  </td>
                  <td className="px-4 py-3 align-top">
                    <div className="font-mono text-xs text-rmb-dark">v{row.version}</div>
                    <div className="text-[11px] text-rmb-gray">
                      {row.version > 1 ? `${row.version} revisions` : "original"}
                    </div>
                  </td>
                  <td className="px-4 py-3 align-top text-rmb-gray">
                    {formatDateTime(row.updated_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <Pagination
        total={total}
        offset={offset}
        limit={limit}
        onPageChange={setOffset}
        onLimitChange={setLimit}
      />
    </div>
  );
}
