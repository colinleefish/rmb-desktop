import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { ChevronRight, ChevronUp } from "lucide-react";
import { getPipelineHealth } from "../lib/api";
import { isPipelineMocked } from "../lib/pipelineMock";
import { formatDateTime } from "../lib/format";
import type {
  PipelineHealth,
  PipelineProblem,
  PipelineStatusCounts,
} from "../lib/types";
import { useI18n } from "../i18n";

const STATUS_KEYS = ["pending", "running", "failed", "idle", "waiting"] as const;

function statusTone(status: string): string {
  switch (status) {
    case "running":
      return "text-rmb-accent";
    case "pending":
      return "text-rmb-accent/80";
    case "failed":
      return "text-red-700";
    case "waiting":
      return "text-rmb-gray";
    case "idle":
    default:
      return "text-rmb-dark";
  }
}

function statusBarClass(status: string): string {
  switch (status) {
    case "running":
      return "bg-rmb-accent";
    case "pending":
      return "bg-rmb-accent/45";
    case "failed":
      return "bg-red-500/80";
    case "waiting":
      return "bg-rmb-gray/15";
    case "idle":
    default:
      return "bg-rmb-gray/30";
  }
}

function stageTotal(counts: PipelineStatusCounts): number {
  return (
    counts.pending + counts.running + counts.failed + counts.idle + counts.waiting
  );
}

function StageStatusCard({
  title,
  hint,
  counts,
  labels,
}: {
  title: string;
  hint: string;
  counts: PipelineStatusCounts;
  labels: Record<(typeof STATUS_KEYS)[number], string>;
}) {
  const total = stageTotal(counts) || 1;

  return (
    <div className="rounded-xl border border-rmb-gray/20 bg-white p-4 shadow-sm">
      <div className="flex items-baseline justify-between gap-2">
        <div>
          <div className="text-sm font-medium text-rmb-dark">{title}</div>
          <div className="text-xs text-rmb-gray">{hint}</div>
        </div>
        <div className="text-2xl font-semibold tabular-nums text-rmb-dark">
          {stageTotal(counts)}
        </div>
      </div>

      <div className="mt-3 flex h-2 overflow-hidden rounded-full bg-rmb-light">
        {STATUS_KEYS.map((key) => {
          const n = counts[key];
          if (!n) return null;
          return (
            <div
              key={key}
              className={statusBarClass(key)}
              style={{ width: `${(n / total) * 100}%` }}
              title={`${labels[key]}: ${n}`}
            />
          );
        })}
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5">
        {STATUS_KEYS.map((key) => (
          <div key={key} className="rounded-md bg-rmb-light px-2 py-1.5">
            <div className="text-[10px] uppercase tracking-wide text-rmb-gray/70">
              {labels[key]}
            </div>
            <div className={`mt-0.5 text-lg font-semibold tabular-nums ${statusTone(key)}`}>
              {counts[key]}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function FunnelStep({
  label,
  value,
  prev,
  widthClass,
}: {
  label: string;
  value: number;
  prev?: number;
  widthClass: string;
}) {
  const drop =
    prev !== undefined && prev > 0 ? Math.round(((prev - value) / prev) * 100) : null;

  return (
    <div className={`mx-auto ${widthClass}`}>
      <div className="rounded-lg border border-rmb-gray/15 bg-rmb-light px-4 py-3 text-center">
        <div className="font-mono text-[10px] uppercase tracking-wider text-rmb-gray/60">
          {label}
        </div>
        <div className="mt-0.5 text-2xl font-semibold tabular-nums text-rmb-dark">
          {value}
        </div>
        {drop !== null && drop > 0 ? (
          <div className="mt-0.5 text-xs text-rmb-gray">−{drop}% from below</div>
        ) : drop === 0 ? (
          <div className="mt-0.5 text-xs text-rmb-gray/50">no drop</div>
        ) : null}
      </div>
    </div>
  );
}

function FunnelArrow() {
  return (
    <ChevronUp
      className="mx-auto size-4 shrink-0 text-rmb-gray/40"
      aria-hidden
    />
  );
}

function ProblemRow({ item }: { item: PipelineProblem }) {
  const failed = item.status.toLowerCase() === "failed";
  return (
    <Link
      to={`/sessions/${encodeURIComponent(item.session_key)}`}
      className="flex items-start justify-between gap-3 border-b border-rmb-gray/10 px-1 py-3 last:border-0 hover:bg-rmb-light/60"
    >
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium text-rmb-dark">
          {item.session_key}
        </div>
        <div className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-rmb-gray">
          <span className="font-mono uppercase">{item.stage}</span>
          <span
            className={
              failed
                ? "rounded bg-red-50 px-1.5 py-0.5 text-red-700"
                : "rounded bg-rmb-accent/10 px-1.5 py-0.5 text-rmb-accent"
            }
          >
            {item.status}
          </span>
          <span>{formatDateTime(item.updated_at)}</span>
        </div>
        {item.reason ? (
          <div className="mt-1 truncate text-xs text-rmb-gray/80">{item.reason}</div>
        ) : null}
      </div>
      <ChevronRight className="mt-1 size-4 shrink-0 text-rmb-gray/40" />
    </Link>
  );
}

export function PipelineHealthPage() {
  const { t } = useI18n();
  const p = t.pipeline;
  const [health, setHealth] = useState<PipelineHealth | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getPipelineHealth()
      .then((data) => {
        if (!cancelled) {
          setHealth(data);
          setError(null);
        }
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading && !health) {
    return <p className="text-rmb-gray">{p.loading}</p>;
  }
  if (error && !health) {
    return <p className="text-red-600">{error}</p>;
  }
  if (!health) {
    return <p className="text-rmb-gray">{p.empty}</p>;
  }

  const statusLabels = {
    pending: p.status.pending,
    running: p.status.running,
    failed: p.status.failed,
    idle: p.status.idle,
    waiting: p.status.waiting,
  };

  const f = health.funnel;

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{p.title}</h1>
        <p className="mt-1 text-rmb-gray">{p.subtitle}</p>
      </div>

      {isPipelineMocked() && (
        <div className="rounded-lg border border-rmb-accent/30 bg-rmb-accent/5 px-3 py-2 text-sm text-rmb-dark">
          {p.previewBanner}
        </div>
      )}

      <section className="rounded-xl border border-rmb-gray/20 bg-white p-4 shadow-sm sm:p-5">
        <div className="flex flex-wrap items-center gap-3">
          <span
            className={[
              "rounded-full px-2.5 py-1 text-xs font-medium",
              health.distillation_enabled
                ? "bg-rmb-accent/15 text-rmb-accent"
                : "bg-red-50 text-red-700",
            ].join(" ")}
          >
            {health.distillation_enabled ? p.enabled : p.disabled}
          </span>
          <span className="text-sm text-rmb-gray">
            <span className="font-semibold tabular-nums text-rmb-dark">
              {health.tracked_sessions}
            </span>{" "}
            {p.countLabel}
          </span>
          <span className="text-xs text-rmb-gray/60">
            {p.updatedAt} {formatDateTime(health.generated_at)}
          </span>
        </div>
        {!health.distillation_enabled ? (
          <p className="mt-3 text-sm text-red-700">{p.disabledHint}</p>
        ) : null}
      </section>

      <section>
        <h2 className="text-lg font-medium">{p.funnelTitle}</h2>
        <p className="text-sm text-rmb-gray">{p.funnelHint}</p>
        {/* Bottom-up: same reading order as Overview distillation pyramid */}
        <div className="mt-4 flex flex-col-reverse items-center gap-2 rounded-xl border border-rmb-gray/20 bg-white p-5 shadow-sm">
          <FunnelStep
            label={p.funnel.sessions}
            value={f.sessions}
            widthClass="w-full max-w-lg"
          />
          <FunnelArrow />
          <FunnelStep
            label={p.funnel.t1}
            value={f.t1_done}
            prev={f.sessions}
            widthClass="w-full max-w-md"
          />
          <FunnelArrow />
          <FunnelStep
            label={p.funnel.t2}
            value={f.t2_done}
            prev={f.t1_done}
            widthClass="w-full max-w-sm"
          />
          <FunnelArrow />
          <FunnelStep
            label={p.funnel.t3}
            value={f.t3_done}
            prev={f.t2_done}
            widthClass="w-full max-w-xs"
          />
        </div>
      </section>

      <section>
        <h2 className="text-lg font-medium">{p.stagesTitle}</h2>
        <p className="text-sm text-rmb-gray">{p.stagesHint}</p>
        <div className="mt-4 grid gap-4 lg:grid-cols-3">
          <StageStatusCard
            title={p.stage.t1}
            hint={p.stage.t1Hint}
            counts={health.stages.t1}
            labels={statusLabels}
          />
          <StageStatusCard
            title={p.stage.t2}
            hint={p.stage.t2Hint}
            counts={health.stages.t2}
            labels={statusLabels}
          />
          <StageStatusCard
            title={p.stage.t3}
            hint={p.stage.t3Hint}
            counts={health.stages.t3}
            labels={statusLabels}
          />
        </div>
      </section>

      <section>
        <h2 className="text-lg font-medium">{p.problemsTitle}</h2>
        <p className="text-sm text-rmb-gray">{p.problemsHint}</p>
        <div className="mt-4 rounded-xl border border-rmb-gray/20 bg-white px-3 shadow-sm sm:px-4">
          {health.problems.length === 0 ? (
            <p className="py-8 text-center text-sm text-rmb-gray">{p.problemsEmpty}</p>
          ) : (
            health.problems.map((item) => (
              <ProblemRow key={`${item.session_key}-${item.stage}-${item.status}`} item={item} />
            ))
          )}
        </div>
      </section>
    </div>
  );
}
