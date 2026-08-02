import { useI18n } from "../i18n";

const PAGE_SIZE_OPTIONS = [10, 25, 50] as const;

export function Pagination({
  total,
  limit,
  offset,
  onPageChange,
  onLimitChange,
}: {
  total: number;
  limit: number;
  offset: number;
  onPageChange: (offset: number) => void;
  onLimitChange?: (limit: number) => void;
}) {
  const { t } = useI18n();
  const page = Math.floor(offset / limit) + 1;
  const pageCount = Math.max(1, Math.ceil(total / limit));
  const from = total === 0 ? 0 : offset + 1;
  const to = Math.min(offset + limit, total);

  if (total === 0) return null;

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-rmb-gray/15 px-4 py-3 text-sm text-rmb-gray">
      <span>
        {t.pagination.showing} {from}–{to} {t.pagination.of} {total}
      </span>

      <div className="flex flex-wrap items-center gap-3">
        {onLimitChange && (
          <label className="flex items-center gap-2">
            <span>{t.pagination.perPage}</span>
            <select
              value={limit}
              onChange={(e) => onLimitChange(Number(e.target.value))}
              className="rounded-md border border-rmb-gray/20 bg-white px-2 py-1 text-sm text-rmb-dark"
            >
              {PAGE_SIZE_OPTIONS.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </label>
        )}

        <div className="flex items-center gap-1">
          <button
            type="button"
            disabled={offset <= 0}
            onClick={() => onPageChange(Math.max(0, offset - limit))}
            className="rounded-md border border-rmb-gray/20 px-3 py-1 text-rmb-dark transition hover:bg-rmb-light disabled:cursor-not-allowed disabled:opacity-40"
          >
            {t.pagination.prev}
          </button>
          <span className="min-w-16 text-center tabular-nums">
            {page} / {pageCount}
          </span>
          <button
            type="button"
            disabled={offset + limit >= total}
            onClick={() => onPageChange(offset + limit)}
            className="rounded-md border border-rmb-gray/20 px-3 py-1 text-rmb-dark transition hover:bg-rmb-light disabled:cursor-not-allowed disabled:opacity-40"
          >
            {t.pagination.next}
          </button>
        </div>
      </div>
    </div>
  );
}

export const DEFAULT_PAGE_SIZE = 25;
