import { Link } from "react-router-dom";
import type { OverviewCounts } from "../lib/types";
import { useI18n } from "../i18n";
import { DistillationPyramid } from "../components/DistillationPyramid";

type StatKey = keyof OverviewCounts;

export function OverviewPage({ counts }: { counts: OverviewCounts }) {
  const { t } = useI18n();

  const groups: {
    title: string;
    caption: string;
    stats: { key: StatKey; href?: string }[];
  }[] = [
    {
      title: t.overview.perSession,
      caption: t.overview.perSessionCaption,
      stats: [
        { key: "sessions", href: "/sessions" },
        { key: "turns" },
        { key: "atoms" },
        { key: "scenes" },
      ],
    },
    {
      title: t.overview.acrossSessions,
      caption: t.overview.acrossSessionsCaption,
      stats: [
        { key: "memories", href: "/memories/profile" },
        { key: "skills", href: "/skills" },
        { key: "corrections" },
      ],
    },
    {
      title: t.overview.workers,
      caption: t.overview.workersCaption,
      stats: [
        { key: "pipeline_states", href: "/pipeline" },
        { key: "tasks" },
      ],
    },
  ];

  const statLabels: Record<StatKey, { label: string; hint: string }> = {
    sessions: { label: t.overview.stats.sessions, hint: t.overview.stats.sessionsHint },
    turns: { label: t.overview.stats.turns, hint: t.overview.stats.turnsHint },
    atoms: { label: t.overview.stats.atoms, hint: t.overview.stats.atomsHint },
    scenes: { label: t.overview.stats.scenes, hint: t.overview.stats.scenesHint },
    memories: { label: t.overview.stats.memories, hint: t.overview.stats.memoriesHint },
    corrections: { label: t.overview.stats.corrections, hint: t.overview.stats.correctionsHint },
    tasks: { label: t.overview.stats.tasks, hint: t.overview.stats.tasksHint },
    pipeline_states: { label: t.overview.stats.pipeline, hint: t.overview.stats.pipelineHint },
    skills: { label: t.overview.stats.skills, hint: t.overview.stats.skillsHint },
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t.overview.title}</h1>
        <p className="mt-1 text-rmb-gray">{t.overview.subtitle}</p>
      </div>

      <DistillationPyramid />

      {groups.map((group) => (
        <section key={group.title}>
          <h2 className="text-lg font-medium">{group.title}</h2>
          <p className="text-sm text-rmb-gray">{group.caption}</p>
          <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {group.stats.map((stat) => {
              const meta = statLabels[stat.key];
              const inner = (
                <div className="rounded-xl border border-rmb-gray/20 bg-white p-4 shadow-sm transition hover:border-rmb-accent/40 hover:shadow">
                  <div className="text-sm text-rmb-gray">{meta.label}</div>
                  <div className="mt-1 text-3xl font-semibold tabular-nums text-rmb-dark">
                    {counts[stat.key] ?? "—"}
                  </div>
                  <div className="mt-2 text-xs text-rmb-gray/60">{meta.hint}</div>
                </div>
              );
              return stat.href ? (
                <Link key={stat.key} to={stat.href}>
                  {inner}
                </Link>
              ) : (
                <div key={stat.key}>{inner}</div>
              );
            })}
          </div>
        </section>
      ))}
    </div>
  );
}
