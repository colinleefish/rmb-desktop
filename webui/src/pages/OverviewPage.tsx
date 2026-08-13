import { Link } from "react-router-dom";
import { ChevronRight, ChevronUp } from "lucide-react";
import type {
  OverviewCounts,
  PipelineHealth,
  PipelineProblem,
  PipelineStatusCounts,
} from "../lib/types";
import { formatDateTime } from "../lib/format";
import { isPipelineMocked } from "../lib/pipelineMock";
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

function KpiCard({
  label,
  value,
  hint,
  href,
}: {
  label: string;
  value: number;
  hint: string;
  href?: string;
}) {
  const inner = (
    <div className="h-full rounded-xl border border-rmb-gray/20 bg-white p-5 shadow-sm transition hover:border-rmb-accent/40 hover:shadow">
      <div className="text-sm text-rmb-gray">{label}</div>
      <div className="mt-2 text-4xl font-semibold tabular-nums text-rmb-dark">
        {value.toLocaleString()}
      </div>
      <div className="mt-2 text-xs text-rmb-gray/60">{hint}</div>
    </div>
  );
  return href ? (
    <Link to={href} className="block h-full">
      {inner}
    </Link>
  ) : (
    inner
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
          {value.toLocaleString()}
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
  return <ChevronUp className="mx-auto size-4 shrink-0 text-rmb-gray/40" aria-hidden />;
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

export function OverviewPage({
  counts,
  health,
}: {
  counts: OverviewCounts;
  health: PipelineHealth;
}) {
  const { t } = useI18n();
  const o = t.overview;
  const p = t.pipeline;

  const workersRunning =
    health.stages.t1.running + health.stages.t2.running + health.stages.t3.running;

  const kpis: { key: keyof OverviewCounts; label: string; hint: string; href?: string }[] = [
    { key: "sessions", label: o.stats.sessions, hint: o.stats.sessionsHint, href: "/sessions" },
    { key: "atoms", label: o.stats.atoms, hint: o.stats.atomsHint },
    { key: "scenes", label: o.stats.scenes, hint: o.stats.scenesHint },
    { key: "memories", label: o.stats.memories, hint: o.stats.memoriesHint, href: "/memories/profile" },
  ];

  const secondary: { key: keyof OverviewCounts; label: string }[] = [
    { key: "turns", label: o.stats.turns },
    { key: "skills", label: o.stats.skills },
    { key: "corrections", label: o.stats.corrections },
  ];

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
        <h1 className="text-2xl font-semibold tracking-tight">{o.title}</h1>
        <p className="mt-1 text-rmb-gray">{o.subtitle}</p>
      </div>

      {isPipelineMocked() && (
        <div className="rounded-lg border border-rmb-accent/30 bg-rmb-accent/5 px-3 py-2 text-sm text-rmb-dark">
          {p.previewBanner}
        </div>
      )}

      {/* Status hero: distillation status, tracked sessions, running workers, snapshot */}
      <section className="rounded-xl border border-rmb-gray/20 bg-white p-5 shadow-sm">
        <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
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

          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-semibold tabular-nums text-rmb-dark">
              {health.tracked_sessions}
            </span>
            <span className="text-sm text-rmb-gray">{p.countLabel}</span>
          </div>

          <div className="flex items-center gap-2">
            <span className="relative flex size-2.5">
              {workersRunning > 0 && (
                <span className="absolute inline-flex size-full animate-ping rounded-full bg-rmb-accent/60" />
              )}
              <span
                className={[
                  "relative inline-flex size-2.5 rounded-full",
                  workersRunning > 0 ? "bg-rmb-accent" : "bg-rmb-gray/40",
                ].join(" ")}
              />
            </span>
            <span className="text-2xl font-semibold tabular-nums text-rmb-dark">
              {workersRunning}
            </span>
            <span className="text-sm text-rmb-gray">{o.workersRunning}</span>
          </div>

          <span className="ml-auto text-xs text-rmb-gray/60">
            {p.updatedAt} {formatDateTime(health.generated_at)}
          </span>
        </div>
        {!health.distillation_enabled ? (
          <p className="mt-3 text-sm text-red-700">{p.disabledHint}</p>
        ) : null}
      </section>

      {/* Key metrics: sessions, atoms, scenes, memories */}
      <section>
        <h2 className="text-lg font-medium">{o.metricsTitle}</h2>
        <p className="text-sm text-rmb-gray">{o.metricsCaption}</p>
        <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {kpis.map((kpi) => (
            <KpiCard
              key={kpi.key}
              label={kpi.label}
              value={counts[kpi.key] ?? 0}
              hint={kpi.hint}
              href={kpi.href}
            />
          ))}
        </div>
        <div className="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-sm text-rmb-gray">
          {secondary.map((stat) => (
            <span key={stat.key} className="flex items-center gap-1.5">
              <span className="font-semibold tabular-nums text-rmb-dark">
                {(counts[stat.key] ?? 0).toLocaleString()}
              </span>
              <span className="text-rmb-gray/70">{stat.label}</span>
            </span>
          ))}
        </div>
      </section>

      {/* Distillation funnel: sessions rise into long-term memory */}
      <section>
        <h2 className="text-lg font-medium">{p.funnelTitle}</h2>
        <p className="text-sm text-rmb-gray">{p.funnelHint}</p>
        <div className="mt-4 flex flex-col-reverse items-center gap-2 rounded-xl border border-rmb-gray/20 bg-white p-5 shadow-sm">
          <FunnelStep label={p.funnel.sessions} value={f.sessions} widthClass="w-full max-w-lg" />
          <FunnelArrow />
          <FunnelStep label={p.funnel.t1} value={f.t1_done} prev={f.sessions} widthClass="w-full max-w-md" />
          <FunnelArrow />
          <FunnelStep label={p.funnel.t2} value={f.t2_done} prev={f.t1_done} widthClass="w-full max-w-sm" />
          <FunnelArrow />
          <FunnelStep label={p.funnel.t3} value={f.t3_done} prev={f.t2_done} widthClass="w-full max-w-xs" />
        </div>
      </section>

      {/* Stage status: worker breakdown per tier */}
      <section>
        <h2 className="text-lg font-medium">{p.stagesTitle}</h2>
        <p className="text-sm text-rmb-gray">{p.stagesHint}</p>
        <div className="mt-4 grid gap-4 lg:grid-cols-3">
          <StageStatusCard title={p.stage.t1} hint={p.stage.t1Hint} counts={health.stages.t1} labels={statusLabels} />
          <StageStatusCard title={p.stage.t2} hint={p.stage.t2Hint} counts={health.stages.t2} labels={statusLabels} />
          <StageStatusCard title={p.stage.t3} hint={p.stage.t3Hint} counts={health.stages.t3} labels={statusLabels} />
        </div>
      </section>

      {/* Needs attention: failed + long-waiting work */}
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
