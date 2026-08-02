import { ChevronUp } from "lucide-react";
import { useI18n } from "../i18n";

function TierCard({
  tier,
  name,
  sub,
  className = "",
}: {
  tier: string;
  name: string;
  sub: string;
  className?: string;
}) {
  return (
    <div
      className={`rounded-lg border border-rmb-gray/15 bg-rmb-light px-4 py-3 text-center ${className}`}
    >
      {tier ? (
        <div className="font-mono text-[10px] uppercase tracking-wider text-rmb-gray/60">
          {tier}
        </div>
      ) : null}
      <div className="mt-0.5 text-sm font-medium text-rmb-dark">{name}</div>
      <div className="mt-0.5 text-xs text-rmb-gray">{sub}</div>
    </div>
  );
}

function TierArrow() {
  return (
    <ChevronUp
      className="mx-auto size-4 shrink-0 text-rmb-gray/40"
      aria-hidden
    />
  );
}

export function DistillationPyramid() {
  const { t } = useI18n();
  const p = t.overview.pyramidChart;

  return (
    <div className="rounded-xl border border-rmb-gray/20 bg-white p-6">
      <div className="text-sm font-medium text-rmb-gray">{t.overview.pyramid}</div>
      <p className="mt-1 text-xs text-rmb-gray/60">{p.caption}</p>

      {/* Bottom-up: flex-col-reverse puts sessions at the bottom, T3 on top */}
      <div className="mt-5 flex flex-col-reverse items-center gap-2">
        <TierCard
          tier={p.sessions.tier}
          name={p.sessions.name}
          sub={p.sessions.sub}
          className="w-full max-w-lg"
        />

        <TierArrow />

        <TierCard
          tier={p.atoms.tier}
          name={p.atoms.name}
          sub={p.atoms.sub}
          className="w-full max-w-md"
        />

        <TierArrow />

        <TierCard
          tier={p.scenes.tier}
          name={p.scenes.name}
          sub={p.scenes.sub}
          className="w-full max-w-sm"
        />

        <TierArrow />

        <div className="w-full max-w-2xl space-y-2">
          <div className="text-center font-mono text-[10px] uppercase tracking-wider text-rmb-gray/60">
            {p.memories.tier}
          </div>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {p.memories.entities.map((entity) => (
              <TierCard
                key={entity.name}
                tier=""
                name={entity.name}
                sub={entity.sub}
                className="px-3 py-2"
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
