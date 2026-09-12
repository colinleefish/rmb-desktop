import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { ChevronRight } from "lucide-react";
import type {
  Overview,
  PipelineHealth,
  PipelineProblem,
  PipelineStatusCounts,
  SessionRow,
} from "../lib/types";
import { formatDateTime, formatDateTimeMonoClass } from "../lib/format";
import { isPipelineMocked } from "../lib/pipelineMock";
import { isFullMock } from "../lib/mockMode";
import { useI18n } from "../i18n";
import { Disclosure } from "../components/Disclosure";
import { EmptyState } from "../components/EmptyState";
import { PipelineFunnelGuide } from "../components/PipelineFunnelGuide";
import { SectionHeader } from "../components/PageHeader";
import {
  attentionListRowGridClass,
  ListRowTextStack,
  ListRowTimestamp,
  SessionListRow,
} from "../components/SessionListRow";
import { StatusPill } from "../components/StatusPill";

const STATUS_KEYS = ["pending", "running", "failed", "idle", "waiting"] as const;
type StatusKey = (typeof STATUS_KEYS)[number];

function stageTotal(counts: PipelineStatusCounts): number {
  return counts.pending + counts.running + counts.failed + counts.idle + counts.waiting;
}

function statusTone(status: StatusKey): string {
  switch (status) {
    case "running":
      return "text-rmb-accent";
    case "failed":
      return "text-rmb-danger";
    case "waiting":
    case "pending":
      return "text-rmb-muted";
    default:
      return "text-rmb-dark";
  }
}

/** One of the three loop tiles: label + big number + a line of context. */
function LoopTile({
  label,
  hint,
  value,
  href,
  children,
}: {
  label: string;
  hint: string;
  value: number;
  href: string;
  children?: ReactNode;
}) {
  return (
    <Link
      to={href}
      className="group flex flex-col rounded-md border border-rmb-line p-4 transition-colors hover:border-rmb-line-strong"
    >
      <div className="flex items-baseline justify-between gap-2">
        <span className="text-sm font-semibold text-rmb-dark">{label}</span>
        <ChevronRight className="size-4 text-rmb-faint opacity-0 transition-opacity group-hover:opacity-100" />
      </div>
      <div className="mt-3 text-[28px] font-semibold leading-none tracking-tight text-rmb-dark">
        {value.toLocaleString()}
      </div>
      <div className="mt-1 text-xs text-rmb-muted">{hint}</div>
      {children ? (
        <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs text-rmb-muted">
          {children}
        </div>
      ) : null}
    </Link>
  );
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <span className="inline-flex items-baseline gap-1">
      <span className="font-medium text-rmb-dark">{value.toLocaleString()}</span>
      <span>{label}</span>
    </span>
  );
}

function ProblemRow({ item }: { item: PipelineProblem }) {
  const failed = item.status.toLowerCase() === "failed";
  return (
    <Link
      to={`/sessions/${encodeURIComponent(item.session_key)}`}
      className={`${attentionListRowGridClass} transition-colors hover:bg-rmb-fill`}
    >
      <div className="flex w-full min-w-0 items-center justify-start">
        <StatusPill tone={failed ? "danger" : "attention"}>
          {item.stage.toUpperCase()} {item.status}
        </StatusPill>
      </div>
      <ListRowTextStack primary={item.session_key} secondary={item.reason} />
      <ListRowTimestamp at={item.updated_at} />
    </Link>
  );
}

function StageRow({
  title,
  hint,
  counts,
  labels,
}: {
  title: string;
  hint: string;
  counts: PipelineStatusCounts;
  labels: Record<StatusKey, string>;
}) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_repeat(5,4rem)] items-baseline gap-3 py-2 text-xs">
      <div className="min-w-0">
        <div className="truncate text-sm font-medium text-rmb-dark">{title}</div>
        <div className="truncate text-rmb-faint">{hint}</div>
      </div>
      {STATUS_KEYS.map((key) => (
        <div key={key} className="text-right">
          <div className={`text-sm font-medium ${statusTone(key)}`}>{counts[key]}</div>
          <div className="text-rmb-faint">{labels[key]}</div>
        </div>
      ))}
    </div>
  );
}

export function OverviewPage({
  overview,
  health,
  recent,
}: {
  overview: Overview;
  health: PipelineHealth;
  recent: SessionRow[];
}) {
  const { t } = useI18n();
  const o = t.overview;
  const p = t.pipeline;
  const counts = overview.counts;
  const byCat = overview.memory_by_category;
  const f = health.funnel;

  const workersRunning =
    health.stages.t1.running + health.stages.t2.running + health.stages.t3.running;

  const statusLabels: Record<StatusKey, string> = {
    pending: p.status.pending,
    running: p.status.running,
    failed: p.status.failed,
    idle: p.status.idle,
    waiting: p.status.waiting,
  };

  const attentionCount = health.problems.length + (health.distillation_enabled ? 0 : 1);

  return (
    <div className="space-y-8">
      {isPipelineMocked() && !isFullMock() && (
        <p className="rounded-md bg-rmb-warn-soft px-3 py-2 text-sm text-rmb-warn">{p.previewBanner}</p>
      )}

      {/* The loop: capture → distill → memories */}
      <section className="grid gap-3 md:grid-cols-3">
        <LoopTile
          label={o.loop.capture}
          hint={o.loop.captureHint}
          value={counts.sessions}
          href="/sessions"
        >
          <Stat value={counts.turns} label={o.stats.turns.toLowerCase()} />
          <Stat value={health.tracked_sessions} label={o.loop.tracked} />
        </LoopTile>

        <LoopTile
          label={o.loop.distillation}
          hint={o.loop.distillationHint}
          value={f.t3_done}
          href="/sessions"
        >
          <StatusPill tone={health.distillation_enabled ? "ok" : "danger"}>
            {health.distillation_enabled ? p.enabled : p.disabled}
          </StatusPill>
          <Stat value={counts.atoms} label={o.stats.atoms.toLowerCase()} />
          <Stat value={counts.scenes} label={o.stats.scenes.toLowerCase()} />
          <Stat value={workersRunning} label={o.workersRunning} />
        </LoopTile>

        <LoopTile
          label={o.loop.memories}
          hint={o.loop.memoriesHint}
          value={counts.memories}
          href="/memories"
        >
          <Stat value={byCat.events} label={t.memories.categories.events.nav.toLowerCase()} />
          <Stat
            value={byCat.preferences}
            label={t.memories.categories.preferences.nav.toLowerCase()}
          />
          <Stat value={byCat.entities} label={t.memories.categories.entities.nav.toLowerCase()} />
          <Stat value={counts.corrections} label={o.stats.corrections.toLowerCase()} />
          <Stat value={counts.skills} label={o.stats.skills.toLowerCase()} />
        </LoopTile>
      </section>

      {/* Latest sessions */}
      <section>
        <SectionHeader
          title={o.loop.latestSessions}
          action={{ to: "/sessions", label: t.common.viewAll }}
        />
        {recent.length === 0 ? (
          <EmptyState title={t.sessions.empty} />
        ) : (
          <div className="divide-y divide-rmb-line overflow-hidden rounded-md border border-rmb-line">
            {recent.map((row) => (
              <SessionListRow key={row.id} row={row} />
            ))}
          </div>
        )}
      </section>

      {/* Needs attention */}
      <section>
        <SectionHeader
          title={`${p.problemsTitle}${attentionCount ? ` · ${attentionCount}` : ""}`}
          hint={p.problemsHint}
        />
        {attentionCount === 0 ? (
          <EmptyState title={p.problemsEmpty} />
        ) : (
          <div className="divide-y divide-rmb-line overflow-hidden rounded-md border border-rmb-line">
            {!health.distillation_enabled ? (
              <Link
                to="/settings/models"
                className={`${attentionListRowGridClass} transition-colors hover:bg-rmb-fill`}
              >
                <div className="flex w-full min-w-0 items-center justify-start">
                  <StatusPill tone="danger">{p.disabled}</StatusPill>
                </div>
                <ListRowTextStack primary={p.disabledHint} primaryMono={false} />
                <span className="text-right text-xs font-medium text-rmb-accent">
                  {o.loop.distillationOffAction}
                </span>
              </Link>
            ) : null}
            {health.problems.map((item) => (
              <ProblemRow key={`${item.session_key}-${item.stage}-${item.status}`} item={item} />
            ))}
          </div>
        )}
      </section>

      {/* Internals, collapsed */}
      <Disclosure summary={t.common.pipelineDetails} hint={o.loop.pipelineHint}>
        <div className="space-y-6">
          <PipelineFunnelGuide
            funnel={f}
            copy={{
              title: p.funnelTitle,
              caption: p.funnelGuide.caption,
              tierLegend: p.funnelGuide.tierLegend,
              steps: p.funnelGuide.steps,
            }}
          />

          <div>
            <div className="text-xs font-medium text-rmb-muted">{p.stagesTitle}</div>
            <p className="mt-1 text-[13px] text-rmb-muted">{p.stagesHint}</p>
            <div className="mt-2 divide-y divide-rmb-line">
              <StageRow
                title={`${p.funnelGuide.steps.t1.name} · T1`}
                hint={p.stage.t1Hint}
                counts={health.stages.t1}
                labels={statusLabels}
              />
              <StageRow
                title={`${p.funnelGuide.steps.t2.name} · T2`}
                hint={p.stage.t2Hint}
                counts={health.stages.t2}
                labels={statusLabels}
              />
              <StageRow
                title={`${p.funnelGuide.steps.t3.name} · T3`}
                hint={p.stage.t3Hint}
                counts={health.stages.t3}
                labels={statusLabels}
              />
            </div>
          </div>

          <p className="text-xs text-rmb-faint">
            {p.updatedAt}{" "}
            <span className={formatDateTimeMonoClass}>{formatDateTime(health.generated_at)}</span>
            {" · "}
            {health.tracked_sessions} {p.countLabel}
            {" · "}
            {stageTotal(health.stages.t1)} / {stageTotal(health.stages.t2)} / {stageTotal(health.stages.t3)}
          </p>
        </div>
      </Disclosure>
    </div>
  );
}
