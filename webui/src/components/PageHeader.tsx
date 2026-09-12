import type { ReactNode } from "react";
import { ChevronLeft } from "lucide-react";
import { Link } from "react-router-dom";

/** Header for detail routes (the Topbar owns list-page titles). */
export function PageHeader({
  back,
  icon,
  title,
  description,
  meta,
  actions,
  mono = false,
}: {
  back?: { to: string; label: string };
  /** Optional identity glyph (e.g. agent logo) shown left of the title. */
  icon?: ReactNode;
  title: string;
  description?: string | null;
  meta?: ReactNode;
  actions?: ReactNode;
  mono?: boolean;
}) {
  return (
    <header className="space-y-2 border-b border-rmb-line pb-5">
      {back ? (
        <Link
          to={back.to}
          className="inline-flex items-center gap-0.5 text-xs text-rmb-muted transition-colors hover:text-rmb-dark"
        >
          <ChevronLeft className="size-3.5" aria-hidden />
          {back.label}
        </Link>
      ) : null}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          {icon}
          <div className="min-w-0 flex-1">
            <h1
              className={`truncate font-semibold tracking-tight text-rmb-dark ${
                mono ? "font-mono text-base" : "text-lg"
              }`}
              title={title}
            >
              {title}
            </h1>
            {description ? (
              <p className="mt-1 max-w-3xl text-sm text-rmb-muted">{description}</p>
            ) : null}
          </div>
        </div>
        {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
      </div>
      {meta ? (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-rmb-muted">
          {meta}
        </div>
      ) : null}
    </header>
  );
}

export function SectionHeader({
  title,
  hint,
  action,
}: {
  title: string;
  hint?: string;
  action?: { to: string; label: string };
}) {
  return (
    <div className="mb-3 flex items-baseline justify-between gap-4">
      <div className="min-w-0">
        <h2 className="text-sm font-semibold text-rmb-dark">{title}</h2>
        {hint ? <p className="text-xs text-rmb-muted">{hint}</p> : null}
      </div>
      {action ? (
        <Link
          to={action.to}
          className="shrink-0 text-xs font-medium text-rmb-accent hover:underline"
        >
          {action.label}
        </Link>
      ) : null}
    </div>
  );
}
