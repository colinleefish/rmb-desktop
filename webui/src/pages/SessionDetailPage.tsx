import { useEffect, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { RecallStatsLabel } from "../components/RecallStatsLabel";
import { AgentChip } from "../components/AgentChip";
import { Disclosure } from "../components/Disclosure";
import { EmptyState, ErrorNote, ListSkeleton } from "../components/EmptyState";
import { PageHeader } from "../components/PageHeader";
import { tabClass } from "../lib/tabClass";
import { DistillPill } from "../components/SessionListRow";
import { StatusPill } from "../components/StatusPill";
import { getSession } from "../lib/api";
import type { AtomRow, SceneRow, SessionDetail, TurnRow } from "../lib/types";
import {
  formatDateTime,
  formatDateTimeMonoClass,
  formatTurnMessage,
  parseTurnMessages,
  turnMessagePreview,
  turnRoleLabel,
} from "../lib/format";
import { useI18n } from "../i18n";

type Tab = "turns" | "atoms" | "scenes";

function TierPill({ label, value }: { label: string; value?: string }) {
  return (
    <StatusPill tone="neutral">
      <span className="mr-1 text-rmb-dark">{label}</span>
      {value ?? "—"}
    </StatusPill>
  );
}

function TurnMessage({
  role,
  content,
}: {
  role?: string;
  content: unknown;
}) {
  const { aside, body } = formatTurnMessage(role, content);
  const normalizedRole = (role ?? "").toLowerCase();
  const isUser = normalizedRole === "user";
  const isSystem = normalizedRole === "system" || normalizedRole === "tool";

  if (!aside && !body) return null;

  return (
    <div className="flex flex-col gap-1.5">
      <span
        className={`text-xs font-medium tracking-wide ${
          isUser
            ? "text-rmb-accent"
            : normalizedRole === "assistant"
              ? "text-rmb-dark"
              : "text-rmb-gray"
        }`}
      >
        {turnRoleLabel(role)}
      </span>
      {aside && (
        <p className="pl-3 text-xs italic leading-relaxed text-rmb-gray">{aside}</p>
      )}
      {body && (
        <div
          className={`rounded-md text-sm leading-relaxed whitespace-pre-wrap ${
            isUser
              ? "bg-rmb-fill px-3.5 py-2.5 text-rmb-dark"
              : isSystem
                ? "bg-rmb-fill/70 px-3 py-2 font-mono text-xs text-rmb-muted"
                : "text-rmb-dark"
          }`}
        >
          {body}
        </div>
      )}
    </div>
  );
}

function TurnsTab({ turns }: { turns: TurnRow[] }) {
  const { t } = useI18n();
  if (!turns.length) return <EmptyState title={t.sessionDetail.noTurns} />;

  return (
    <ol className="divide-y divide-rmb-line">
      {turns.map((turn) => {
        const messages = parseTurnMessages(turn.messages_jsonl);
        return (
          <li key={turn.id} className="py-5 first:pt-0">
            <div className="mb-3 flex items-baseline justify-between gap-3">
              <span className="text-sm font-semibold text-rmb-dark">
                {t.sessionDetail.turn} {turn.turn_index + 1}
              </span>
              <span className={`text-xs text-rmb-faint ${formatDateTimeMonoClass}`}>
                {formatDateTime(turn.created_at)}
              </span>
            </div>
            <p className="mb-4 text-xs text-rmb-muted">{turnMessagePreview(messages)}</p>
            {messages.length === 0 ? (
              <p className="text-sm text-rmb-muted">{t.sessionDetail.noMessages}</p>
            ) : (
              <div className="space-y-4">
                {messages.map((msg, idx) => (
                  <TurnMessage key={idx} role={msg.role} content={msg.content} />
                ))}
              </div>
            )}
            <details className="mt-4">
              <summary className="cursor-pointer text-xs text-rmb-faint hover:text-rmb-dark">
                {t.sessionDetail.rawJson}
              </summary>
              <pre className="mt-2 overflow-x-auto rounded-md bg-rmb-fill p-3 font-mono text-xs text-rmb-dark">
                {turn.messages_jsonl}
              </pre>
            </details>
          </li>
        );
      })}
    </ol>
  );
}

function AtomsTab({ atoms }: { atoms: AtomRow[] }) {
  const { t } = useI18n();
  if (!atoms.length) return <EmptyState title={t.sessionDetail.noAtoms} />;

  return (
    <div className="divide-y divide-rmb-line rounded-md border border-rmb-line">
      {atoms.map((atom) => (
        <article key={atom.id} className="px-4 py-3">
          <div className="flex flex-wrap items-center gap-2 text-xs text-rmb-muted">
            <StatusPill tone="info">{atom.category}</StatusPill>
            {atom.scene_name && <span>{atom.scene_name}</span>}
            {atom.slug && <span className="font-mono text-rmb-faint">{atom.slug}</span>}
          </div>
          <p className="mt-1.5 text-sm text-rmb-dark">{atom.content}</p>
        </article>
      ))}
    </div>
  );
}

function ScenesTab({ scenes }: { scenes: SceneRow[] }) {
  const { t } = useI18n();
  if (!scenes.length) return <EmptyState title={t.sessionDetail.noScenes} />;

  return (
    <div className="divide-y divide-rmb-line rounded-md border border-rmb-line">
      {scenes.map((scene) => (
        <article key={scene.id} className="px-4 py-3">
          <div className="flex items-start justify-between gap-3">
            <h3 className="text-sm font-medium text-rmb-dark">
              {scene.display_name ?? scene.id}
            </h3>
            <RecallStatsLabel stats={scene.recall_stats} unit={t.memories.recalls} />
          </div>
          {scene.abstract && <p className="mt-1 text-sm text-rmb-muted">{scene.abstract}</p>}
          {scene.body && (
            <p className="mt-2 whitespace-pre-wrap text-sm text-rmb-dark">{scene.body}</p>
          )}
        </article>
      ))}
    </div>
  );
}

export function SessionDetailPage() {
  const { t } = useI18n();
  const { sessionKey = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = (searchParams.get("tab") as Tab) || "turns";
  const [detail, setDetail] = useState<SessionDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!sessionKey) return;
    setLoading(true);
    setError(null);
    getSession(sessionKey)
      .then(setDetail)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [sessionKey]);

  const setTab = (next: Tab) => {
    setSearchParams({ tab: next }, { replace: true });
  };

  if (!sessionKey) return <EmptyState title={t.sessionDetail.missingKey} />;
  if (loading) return <ListSkeleton rows={5} />;
  if (error) return <ErrorNote>{error}</ErrorNote>;
  if (!detail) return null;

  const { session, pipeline_state: pipeline, turns, atoms, scenes } = detail;
  const t1 = pipeline?.t1_status ?? session.t1_status;
  const t2 = pipeline?.t2_status ?? session.t2_status;
  const t3 = pipeline?.t3_status ?? session.t3_status;
  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: "turns", label: t.sessionDetail.tabs.turns, count: turns.length },
    { id: "atoms", label: t.sessionDetail.tabs.atoms, count: atoms.length },
    { id: "scenes", label: t.sessionDetail.tabs.scenes, count: scenes.length },
  ];

  return (
    <div className="space-y-6">
      <PageHeader
        back={{ to: "/sessions", label: t.sessionDetail.back }}
        title={session.session_key}
        mono
        description={session.abstract}
        meta={
          <>
            <AgentChip source={session.source} />
            <DistillPill t1={t1} t2={t2} t3={t3} />
            <span>
              {session.turn_count}
              {t.sessions.turnUnit}
            </span>
            <span className={`text-rmb-faint ${formatDateTimeMonoClass}`}>
              {formatDateTime(session.last_turn_at ?? session.updated_at)}
            </span>
          </>
        }
      />

      {/* Value funnel for this one session: turns → atoms → scenes */}
      <div className="flex flex-wrap items-baseline gap-x-2 text-sm text-rmb-muted">
        {tabs.map((item, i) => (
          <span key={item.id} className="inline-flex items-baseline gap-1">
            {i > 0 ? <span className="mr-1 text-rmb-faint">→</span> : null}
            <span className="font-semibold text-rmb-dark">{item.count}</span>
            <span>{item.label.toLowerCase()}</span>
          </span>
        ))}
      </div>

      <div className="flex gap-1 border-b border-rmb-line">
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            onClick={() => setTab(item.id)}
            className={tabClass(tab === item.id)}
          >
            {item.label}
            <span className="text-[11px] text-rmb-faint">{item.count}</span>
          </button>
        ))}
      </div>

      {tab === "turns" && <TurnsTab turns={turns} />}
      {tab === "atoms" && <AtomsTab atoms={atoms} />}
      {tab === "scenes" && <ScenesTab scenes={scenes} />}

      <Disclosure summary={t.common.pipelineDetails}>
        <div className="flex flex-wrap gap-2">
          <TierPill label="T1" value={t1} />
          <TierPill label="T2" value={t2} />
          <TierPill label="T3" value={t3} />
          {pipeline ? (
            <span className={`ml-2 self-center text-xs text-rmb-faint ${formatDateTimeMonoClass}`}>
              {formatDateTime(pipeline.updated_at)}
            </span>
          ) : null}
        </div>
      </Disclosure>
    </div>
  );
}
