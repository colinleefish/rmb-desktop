import type { ReactNode } from "react";
import { Link } from "react-router-dom";

/** Empty list: one line of copy and, optionally, the single next action. */
export function EmptyState({
  title,
  hint,
  action,
}: {
  title: string;
  hint?: string;
  action?: { to: string; label: string };
}) {
  return (
    <div className="rounded-md border border-dashed border-rmb-line px-4 py-10 text-center">
      <p className="text-sm text-rmb-dark">{title}</p>
      {hint ? <p className="mt-1 text-xs text-rmb-muted">{hint}</p> : null}
      {action ? (
        <Link
          to={action.to}
          className="mt-3 inline-block text-sm font-medium text-rmb-accent hover:underline"
        >
          {action.label}
        </Link>
      ) : null}
    </div>
  );
}

export function ErrorNote({ children }: { children: ReactNode }) {
  return (
    <div
      role="alert"
      className="rounded-md bg-rmb-danger-soft px-3 py-2 text-sm text-rmb-danger"
    >
      {children}
    </div>
  );
}

/** Skeleton rows shaped like a list row (56px). */
export function ListSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div aria-hidden className="divide-y divide-rmb-line">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex h-14 items-center gap-4 px-3">
          <span className="h-3 w-16 rounded bg-rmb-fill" />
          <span className="h-3 flex-1 rounded bg-rmb-fill" />
          <span className="h-3 w-12 rounded bg-rmb-fill" />
          <span className="h-3 w-20 rounded bg-rmb-fill" />
        </div>
      ))}
    </div>
  );
}
