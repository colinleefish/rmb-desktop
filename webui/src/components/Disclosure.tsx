import type { ReactNode } from "react";
import { ChevronRight } from "lucide-react";

/** Collapsed-by-default section for internals the user rarely needs (pipeline tiers, raw state). */
export function Disclosure({
  summary,
  hint,
  children,
  defaultOpen = false,
}: {
  summary: string;
  hint?: string;
  children: ReactNode;
  defaultOpen?: boolean;
}) {
  return (
    <details className="group rounded-md border border-rmb-line" open={defaultOpen}>
      <summary className="flex cursor-pointer select-none items-center gap-2 px-4 py-3 text-sm font-medium text-rmb-dark [&::-webkit-details-marker]:hidden">
        <ChevronRight
          className="size-4 shrink-0 text-rmb-faint transition-transform group-open:rotate-90"
          aria-hidden
        />
        <span>{summary}</span>
        {hint ? <span className="truncate text-xs font-normal text-rmb-faint">{hint}</span> : null}
      </summary>
      <div className="border-t border-rmb-line px-4 py-4">{children}</div>
    </details>
  );
}
