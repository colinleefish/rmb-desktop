import { useEffect, useState, type ReactNode } from "react";
import { Navigate, useLocation, useParams, useSearchParams } from "react-router-dom";
import { MemoryCorrections } from "../components/MemoryCorrections";
import { PageTabs } from "../components/PageTabs";
import { memoryTitle, MemoryListRow } from "../components/MemoryListRow";
import { RecallStatsLabel } from "../components/RecallStatsLabel";
import { Modal } from "../components/Modal";
import { DEFAULT_PAGE_SIZE, Pagination } from "../components/Pagination";
import { EmptyState, ErrorNote, ListSkeleton } from "../components/EmptyState";
import { pageMemories } from "../lib/api";
import {
  MEMORY_CATEGORIES,
  isMemoryCategory,
  type MemoryCategory,
} from "../lib/memoryCategories";
import { formatDateTime, formatDateTimeMonoClass } from "../lib/format";
import type { MemoryRow } from "../lib/types";
import { useI18n } from "../i18n";
import { MemoryMarkdown } from "../components/MemoryMarkdown";
import { getMemoriesSectionMeta } from "../lib/shellRoutes";

/** Meta line shared by the modal and the profile article: version · updated · recalls. */
function MemoryMeta({ memory }: { memory: MemoryRow }) {
  const { t } = useI18n();
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-rmb-muted">
      <span className="inline-flex items-center shrink-0">v{memory.version}</span>
      <span className={`inline-flex items-center ${formatDateTimeMonoClass}`}>
        {formatDateTime(memory.updated_at)}
      </span>
      <RecallStatsLabel
        stats={memory.recall_stats}
        unit={t.memories.recalls}
        uniform
        className="inline-flex shrink-0 items-center"
      />
    </div>
  );
}

function MemoryBody({ memory, emptyLabel }: { memory: MemoryRow; emptyLabel: string }) {
  return (
    <>
      {memory.abstract && <p className="text-sm text-rmb-dark">{memory.abstract}</p>}
      {memory.body ? (
        <pre className="mt-4 whitespace-pre-wrap font-sans text-sm leading-relaxed text-rmb-muted">
          {memory.body}
        </pre>
      ) : (
        <p className="mt-4 text-sm text-rmb-faint">{emptyLabel}</p>
      )}
    </>
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
    <Modal open={!!memory} onClose={onClose} title={memoryTitle(memory)} subtitle={memory.uri}>
      <MemoryBody memory={memory} emptyLabel="—" />
      <div className="mt-6">
        <MemoryMeta memory={memory} />
      </div>
      <MemoryCorrections memoryURI={memory.uri} />
    </Modal>
  );
}

function ProfileMemoryBody({ memory, emptyLabel }: { memory: MemoryRow; emptyLabel: string }) {
  return (
    <>
      {memory.abstract ? (
        <div className="rounded-lg border border-rmb-line bg-rmb-fill/70 px-5 py-4">
          <p className="text-[15px] leading-relaxed text-rmb-dark">{memory.abstract}</p>
        </div>
      ) : null}
      {memory.body ? (
        <div className={memory.abstract ? "mt-8" : undefined}>
          <MemoryMarkdown content={memory.body} />
        </div>
      ) : (
        <p className={`text-sm text-rmb-faint ${memory.abstract ? "mt-6" : ""}`}>{emptyLabel}</p>
      )}
    </>
  );
}

function ProfileMemoryView({ memory }: { memory: MemoryRow }) {
  const { t } = useI18n();
  return (
    <article className="w-full max-w-5xl">
      <ProfileMemoryBody memory={memory} emptyLabel={t.memories.emptyProfile} />
      <footer className="mt-8 border-t border-rmb-line pt-4">
        <MemoryMeta memory={memory} />
      </footer>
      <MemoryCorrections memoryURI={memory.uri} />
    </article>
  );
}

type SortKey = "updated" | "search" | "version";

function MemoryListView({
  category,
  showCategory,
}: {
  category?: MemoryCategory;
  showCategory?: boolean;
}) {
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
  const query = searchParams.get("q") ?? "";
  const [rows, setRows] = useState<MemoryRow[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(DEFAULT_PAGE_SIZE);
  const [selected, setSelected] = useState<MemoryRow | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [sort, setSort] = useState<SortKey>("updated");

  useEffect(() => {
    setLoading(true);
    pageMemories({
      limit,
      offset,
      category,
      q: query || undefined,
      sort,
      order: "desc",
    })
      .then((page) => {
        setRows(page.items);
        setTotal(page.total);
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [category, query, limit, offset, sort]);

  useEffect(() => {
    setOffset(0);
    setSelected(null);
  }, [query, category, sort]);

  if (loading && !rows.length) return <ListSkeleton rows={6} />;
  if (error) return <ErrorNote>{error}</ErrorNote>;

  const categoryLabel = (cat: string) =>
    isMemoryCategory(cat) ? t.memories.categories[cat].nav : cat;

  const sortOptions: { value: SortKey; label: string }[] = [
    { value: "updated", label: t.memories.sortUpdated },
    { value: "search", label: t.memories.sortRecalled },
    { value: "version", label: t.memories.sortVersion },
  ];

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-4 text-xs text-rmb-muted">
        <span>
          {total} {t.memories.countLabel}
          {query ? (
            <>
              {" · "}
              <span className="font-mono text-rmb-dark">“{query}”</span>
            </>
          ) : null}
        </span>
        <label className="flex items-center gap-2">
          <span>{t.common.sortBy}</span>
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value as SortKey)}
            className="h-7 rounded-md border border-rmb-line-strong bg-white px-1.5 text-xs text-rmb-dark"
          >
            {sortOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>
      </div>

      {rows.length === 0 ? (
        <EmptyState
          title={query ? t.memories.emptyCategory : t.memories.empty}
          action={query ? undefined : { to: "/settings/models", label: t.settings.tabs.models }}
        />
      ) : (
        <div className="overflow-hidden rounded-md border border-rmb-line">
          <div className="divide-y divide-rmb-line">
            {rows.map((memory) => (
              <MemoryListRow
                key={memory.id}
                memory={memory}
                showCategory={showCategory}
                categoryLabel={showCategory ? categoryLabel : undefined}
                onSelect={() => setSelected(memory)}
              />
            ))}
          </div>
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

  if (loading) return <ListSkeleton rows={4} />;
  if (error) return <ErrorNote>{error}</ErrorNote>;

  return memory ? (
    <ProfileMemoryView memory={memory} />
  ) : (
    <EmptyState title={t.memories.emptyProfile} />
  );
}

function MemoriesTabHeader({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <header className="space-y-1">
      <h2 className="text-lg font-semibold tracking-tight text-rmb-dark">{title}</h2>
      <p className="max-w-3xl text-sm text-rmb-muted">{subtitle}</p>
    </header>
  );
}

function MemoriesChrome({ children }: { children: ReactNode }) {
  const { t } = useI18n();
  const location = useLocation();
  const tabMeta = getMemoriesSectionMeta(location.pathname, t);

  const q = location.search;
  const tabs = [
    { to: { pathname: "/memories", search: q }, label: t.memories.tabAll, end: true },
    ...MEMORY_CATEGORIES.map((category) => ({
      to: { pathname: `/memories/${category}`, search: q },
      label: t.memories.categories[category].nav,
    })),
  ];

  return (
    <div className="space-y-5">
      <PageTabs tabs={tabs} />
      {tabMeta ? <MemoriesTabHeader title={tabMeta.title} subtitle={tabMeta.subtitle} /> : null}
      {children}
    </div>
  );
}

export function MemoriesPage() {
  const { category: categoryParam } = useParams<{ category?: string }>();

  if (!categoryParam) {
    return (
      <MemoriesChrome>
        <MemoryListView showCategory />
      </MemoriesChrome>
    );
  }
  if (!isMemoryCategory(categoryParam)) {
    return <Navigate to="/memories" replace />;
  }

  if (categoryParam === "profile") {
    return (
      <MemoriesChrome>
        <ProfileMemoryPage />
      </MemoriesChrome>
    );
  }

  return (
    <MemoriesChrome>
      <MemoryListView category={categoryParam} />
    </MemoriesChrome>
  );
}
