import type { PipelineFunnel } from "../lib/types";

type FunnelGuideCopy = {
  caption: string;
  tierLegend: string;
  title: string;
  steps: {
    sessions: { name: string; sub: string };
    t1: { name: string; sub: string };
    t2: { name: string; sub: string };
    t3: { name: string; sub: string };
  };
};

function dropPct(prev: number, value: number): number | null {
  if (prev <= 0 || value >= prev) return null;
  return Math.round(((prev - value) / prev) * 100);
}

function FunnelStageCard({
  name,
  sub,
  value,
  drop,
}: {
  name: string;
  sub: string;
  value: number;
  drop: number | null;
}) {
  return (
    <div className="min-w-[5.5rem] flex-1 rounded-md border border-rmb-line px-3 py-2.5">
      <div className="truncate text-sm font-medium text-rmb-dark">{name}</div>
      <div className="mt-1 text-xl font-semibold tabular-nums leading-none tracking-tight text-rmb-dark">
        {value.toLocaleString()}
      </div>
      <div className="mt-1 truncate text-xs text-rmb-muted">{sub}</div>
      {drop !== null && drop > 0 ? (
        <div className="mt-0.5 text-[11px] text-rmb-faint">−{drop}%</div>
      ) : null}
    </div>
  );
}

function FlowArrow() {
  return (
    <span className="shrink-0 self-center text-sm text-rmb-faint" aria-hidden>
      →
    </span>
  );
}

/** Distillation funnel inside Pipeline details — sessions → T1 → T2 → T3 with human labels. */
export function PipelineFunnelGuide({
  funnel,
  copy,
}: {
  funnel: PipelineFunnel;
  copy: FunnelGuideCopy;
}) {
  const steps = [
    { key: "sessions", value: funnel.sessions, prev: undefined as number | undefined },
    { key: "t1", value: funnel.t1_done, prev: funnel.sessions },
    { key: "t2", value: funnel.t2_done, prev: funnel.t1_done },
    { key: "t3", value: funnel.t3_done, prev: funnel.t2_done },
  ] as const;

  return (
    <div className="space-y-3">
      <div>
        <div className="text-xs font-medium text-rmb-muted">{copy.title}</div>
        <p className="mt-1 text-[13px] leading-snug text-rmb-muted">{copy.caption}</p>
      </div>

      <div className="flex flex-wrap items-stretch gap-x-1.5 gap-y-2">
        {steps.map((step, i) => {
          const meta = copy.steps[step.key];
          const drop =
            step.prev !== undefined ? dropPct(step.prev, step.value) : null;
          return (
            <span key={step.key} className="contents">
              {i > 0 ? <FlowArrow /> : null}
              <FunnelStageCard
                name={meta.name}
                sub={meta.sub}
                value={step.value}
                drop={drop}
              />
            </span>
          );
        })}
      </div>

      <p className="text-xs text-rmb-faint">{copy.tierLegend}</p>
    </div>
  );
}
