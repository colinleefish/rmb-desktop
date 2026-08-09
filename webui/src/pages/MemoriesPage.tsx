import { useEffect, useState } from "react";
import { Navigate, useParams } from "react-router-dom";
import { MemoryCorrections } from "../components/MemoryCorrections";
import { RecallStatsLabel } from "../components/RecallStatsLabel";
import { Modal } from "../components/Modal";
import { DEFAULT_PAGE_SIZE, Pagination } from "../components/Pagination";
import { pageMemories } from "../lib/api";
import {
  DEFAULT_MEMORY_CATEGORY,
  isMemoryCategory,
  type MemoryCategory,
} from "../lib/memoryCategories";
import { formatDateTime } from "../lib/format";
import type { MemoryRow } from "../lib/types";
import { useI18n } from "../i18n";

function memoryTitle(memory: MemoryRow): string {
  return (
    memory.slug?.replace(/[-_]+/g, " ") ??
    memory.category.charAt(0).toUpperCase() + memory.category.slice(1)
  );
}

function MemoryDetailModal({
  memory,
  onClose,
}: {
  memory: MemoryRow | null;
  onClose: () => void;
}) {
  if (!memory) return null;

  return (
    <Modal
      open={!!memory}
      onClose={onClose}
      title={memoryTitle(memory)}
      subtitle={memory.uri}
    >
      {memory.abstract && (
        <p className="text-sm text-rmb-dark">{memory.abstract}</p>
      )}
      {memory.body ? (
        <pre className="mt-4 whitespace-pre-wrap font-sans text-sm leading-relaxed text-rmb-gray">
          {memory.body}
        </pre>
      ) : (
        <p className="mt-4 text-sm text-rmb-gray">—</p>
      )}
      <p className="mt-6 text-xs text-rmb-gray">
        v{memory.version} · {formatDateTime(memory.updated_at)}
        {" · "}
        <RecallStatsLabel stats={memory.recall_stats} />
      </p>
      <MemoryCorrections memoryURI={memory.uri} />
    </Modal>
  );
}

function ProfileMemoryView({ memory }: { memory: MemoryRow }) {
  const { t } = useI18n();
  return (
    <article className="rounded-xl border border-rmb-gray/20 bg-white p-6">
      {memory.abstract && (
        <p className="text-sm text-rmb-gray">{memory.abstract}</p>
      )}
      {memory.body ? (
        <pre className="mt-4 whitespace-pre-wrap font-sans text-sm leading-relaxed text-rmb-dark">
          {memory.body}
        </pre>
      ) : (
        <p className="mt-4 text-sm text-rmb-gray">{t.memories.emptyProfile}</p>
      )}
      <p className="mt-6 text-xs text-rmb-gray">
        v{memory.version} · {formatDateTime(memory.updated_at)}
        {" · "}
        <RecallStatsLabel stats={memory.recall_stats} />
      </p>
      <MemoryCorrections memoryURI={memory.uri} />
    </article>
  );
}

function MemoryListView({
  category,
  title,
  subtitle,
}: {
  category: MemoryCategory;
  title: string;
  subtitle: string;
}) {
  const { t } = useI18n();
  const [rows, setRows] = useState<MemoryRow[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(DEFAULT_PAGE_SIZE);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<MemoryRow | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    pageMemories({
      limit,
      offset,
      category,
      q: query || undefined,
      sort: "updated",
      order: "desc",
    })
      .then((page) => {
        setRows(page.items);
        setTotal(page.total);
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [category, query, limit, offset]);

  useEffect(() => {
    setOffset(0);
    setSelected(null);
  }, [query, category]);

  if (loading && !rows.length) {
    return <p className="text-rmb-gray">{t.memories.loading}</p>;
  }
  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold text-rmb-dark">{title}</h1>
        <p className="mt-1 text-rmb-gray">{subtitle}</p>
      </div>

      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder={t.memories.search}
        className="w-full max-w-md rounded-md border border-rmb-gray/20 px-3 py-2 text-sm text-rmb-dark"
      />

      <div className="overflow-x-auto rounded-xl border border-rmb-gray/20 bg-white">
        {rows.length === 0 ? (
          <p className="px-4 py-8 text-center text-rmb-gray">{t.memories.emptyCategory}</p>
        ) : (
          <table className="w-full table-fixed text-left text-sm">
            <colgroup>
              <col className="w-[34%]" />
              <col />
              <col className="w-16" />
              <col className="w-24" />
              <col className="w-40" />
            </colgroup>
            <thead className="border-b border-rmb-gray/15 bg-rmb-light text-rmb-gray">
              <tr>
                <th className="px-4 py-3 font-medium">{t.memories.colTitle}</th>
                <th className="px-4 py-3 font-medium">{t.memories.colAbstract}</th>
                <th className="px-4 py-3 font-medium">{t.memories.colVersion}</th>
                <th className="px-4 py-3 font-medium">{t.memories.colRecall}</th>
                <th className="px-4 py-3 font-medium">{t.memories.colUpdated}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((memory) => (
                <tr
                  key={memory.id}
                  className="h-[4.5rem] cursor-pointer border-b border-rmb-gray/10 transition last:border-0 hover:bg-rmb-light/60"
                  onClick={() => setSelected(memory)}
                >
                  <td className="px-4 align-middle">
                    <div className="min-w-0">
                      <span
                        className="block truncate font-medium text-rmb-dark"
                        title={memoryTitle(memory)}
                      >
                        {memoryTitle(memory)}
                      </span>
                      <span
                        className="mt-0.5 block truncate font-mono text-xs text-rmb-gray/50"
                        title={memory.uri}
                      >
                        {memory.uri}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 align-middle">
                    <div className="flex h-10 items-center">
                      <p
                        className="line-clamp-2 text-sm leading-5 text-rmb-gray"
                        title={memory.abstract ?? undefined}
                      >
                        {memory.abstract ?? "—"}
                      </p>
                    </div>
                  </td>
                  <td className="px-4 align-middle tabular-nums text-rmb-gray">
                    v{memory.version}
                  </td>
                  <td className="px-4 align-middle">
                    <RecallStatsLabel stats={memory.recall_stats} />
                  </td>
                  <td className="px-4 align-middle text-xs text-rmb-gray">
                    {formatDateTime(memory.updated_at)}
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

      <MemoryDetailModal memory={selected} onClose={() => setSelected(null)} />
    </div>
  );
}

function ProfileMemoryPage() {
  const { t } = useI18n();
  const [memory, setMemory] = useState<MemoryRow | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    pageMemories({
      limit: 1,
      offset: 0,
      category: "profile",
      sort: "updated",
      order: "desc",
    })
      .then((page) => setMemory(page.items[0] ?? null))
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <p className="text-rmb-gray">{t.memories.loading}</p>;
  }
  if (error) return <p className="text-red-600">{error}</p>;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold text-rmb-dark">
          {t.memories.categories.profile.title}
        </h1>
        <p className="mt-1 text-rmb-gray">{t.memories.categories.profile.subtitle}</p>
      </div>
      {memory ? (
        <ProfileMemoryView memory={memory} />
      ) : (
        <p className="rounded-xl border border-rmb-gray/20 bg-white px-4 py-8 text-center text-rmb-gray">
          {t.memories.emptyProfile}
        </p>
      )}
    </div>
  );
}

export function MemoriesPage() {
  const { category: categoryParam } = useParams<{ category?: string }>();
  const { t } = useI18n();

  if (!categoryParam) {
    return <Navigate to={`/memories/${DEFAULT_MEMORY_CATEGORY}`} replace />;
  }
  if (!isMemoryCategory(categoryParam)) {
    return <Navigate to={`/memories/${DEFAULT_MEMORY_CATEGORY}`} replace />;
  }

  if (categoryParam === "profile") {
    return <ProfileMemoryPage />;
  }

  const meta = t.memories.categories[categoryParam];
  return (
    <MemoryListView
      category={categoryParam}
      title={meta.title}
      subtitle={meta.subtitle}
    />
  );
}
