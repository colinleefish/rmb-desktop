import type { MemoryRow } from "../lib/types";
import { formatDateTime, formatDateTimeMonoClass } from "../lib/format";
import { useI18n } from "../i18n";
import { RecallStatsLabel } from "./RecallStatsLabel";
import { StatusPill } from "./StatusPill";

export function memoryTitle(memory: MemoryRow): string {
  return (
    memory.slug?.replace(/[-_]+/g, " ") ??
    memory.category.charAt(0).toUpperCase() + memory.category.slice(1)
  );
}

/** Wide (xl+): title+URI · abstract · optional category · recalls · updated. Narrow: title+URI · clamped abstract · time. */
export function memoryListRowGridClass(showCategory: boolean) {
  return showCategory
    ? "xl:grid-cols-[minmax(10rem,0.95fr)_minmax(12rem,2fr)_6rem_5rem_11.5rem]"
    : "xl:grid-cols-[minmax(10rem,0.95fr)_minmax(12rem,2fr)_5rem_11.5rem]";
}

const abstractPreviewClass =
  "line-clamp-2 overflow-hidden break-words text-xs leading-snug text-rmb-muted";

const rowBaseClass =
  "grid min-h-14 w-full min-w-0 grid-cols-1 items-center gap-1 overflow-hidden px-3 py-2 text-left transition-colors hover:bg-rmb-fill xl:gap-4";

export function MemoryListRow({
  memory,
  showCategory,
  categoryLabel,
  onSelect,
}: {
  memory: MemoryRow;
  showCategory?: boolean;
  categoryLabel?: (cat: string) => string;
  onSelect: () => void;
}) {
  const { t } = useI18n();
  const title = memoryTitle(memory);
  const updatedAt = formatDateTime(memory.updated_at);

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`${rowBaseClass} ${memoryListRowGridClass(!!showCategory)}`}
    >
      <div className="min-w-0 max-w-full shrink-0 xl:min-w-0">
        <div className="truncate text-sm font-medium text-rmb-dark" title={title}>
          {title}
        </div>
        <div className="truncate font-mono text-xs text-rmb-faint" title={memory.uri}>
          {memory.uri}
        </div>
        {memory.abstract ? (
          <p
            className={`mt-0.5 ${abstractPreviewClass} xl:hidden`}
            title={memory.abstract}
          >
            {memory.abstract}
          </p>
        ) : null}
        <span
          className={`mt-0.5 block truncate text-xs text-rmb-faint xl:hidden ${formatDateTimeMonoClass}`}
          title={updatedAt}
        >
          {updatedAt}
        </span>
      </div>

      <div className="hidden min-w-0 max-w-full xl:block">
        {memory.abstract ? (
          <p className={abstractPreviewClass} title={memory.abstract}>
            {memory.abstract}
          </p>
        ) : null}
      </div>

      {showCategory && categoryLabel ? (
        <div className="hidden xl:block">
          <StatusPill tone="neutral">{categoryLabel(memory.category)}</StatusPill>
        </div>
      ) : null}

      <RecallStatsLabel
        stats={memory.recall_stats}
        unit={t.memories.recalls}
        className="hidden justify-end xl:inline-flex"
      />
      <span
        className={`hidden text-right text-xs text-rmb-faint xl:block ${formatDateTimeMonoClass}`}
      >
        {updatedAt}
      </span>
    </button>
  );
}
