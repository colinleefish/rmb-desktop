import { useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { getSession } from "../lib/api";
import type { AtomRow, SceneRow, SessionDetail, TurnRow } from "../lib/types";
import {
  formatDateTime,
  formatTurnMessage,
  parseTurnMessages,
  turnMessagePreview,
  turnRoleLabel,
} from "../lib/format";
import { useI18n } from "../i18n";

type Tab = "turns" | "atoms" | "scenes";

function StatusPill({ label, value }: { label: string; value?: string }) {
  return (
    <span className="inline-flex items-center gap-1 rounded bg-rmb-light px-2 py-0.5 text-xs text-rmb-gray">
      <span className="font-medium text-rmb-dark">{label}</span>
      {value ?? "—"}
    </span>
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
          className={`rounded-lg text-sm leading-relaxed whitespace-pre-wrap ${
            isUser
              ? "border border-rmb-gray/15 bg-rmb-light px-3.5 py-2.5 text-rmb-dark"
              : isSystem
                ? "bg-rmb-light/80 px-3 py-2 font-mono text-xs text-rmb-gray"
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
  if (!turns.length) {
    return <p className="text-sm text-rmb-gray">{t.sessionDetail.noTurns}</p>;
  }

  return (
    <ol className="space-y-6">
      {turns.map((turn) => {
        const messages = parseTurnMessages(turn.messages_jsonl);
        return (
          <li key={turn.id} className="rounded-lg border border-rmb-gray/15 bg-white p-4">
            <div className="mb-3 flex items-baseline justify-between gap-3">
              <span className="text-sm font-medium text-rmb-dark">
                {t.sessionDetail.turn} {turn.turn_index + 1}
              </span>
              <span className="text-xs text-rmb-gray">{formatDateTime(turn.created_at)}</span>
            </div>
            <p className="mb-4 text-xs text-rmb-gray">{turnMessagePreview(messages)}</p>
            {messages.length === 0 ? (
              <p className="text-sm text-rmb-gray">{t.sessionDetail.noMessages}</p>
            ) : (
              <div className="space-y-4">
                {messages.map((msg, idx) => (
                  <TurnMessage key={idx} role={msg.role} content={msg.content} />
                ))}
              </div>
            )}
            <details className="mt-4">
              <summary className="cursor-pointer text-xs text-rmb-gray hover:text-rmb-dark">
                {t.sessionDetail.rawJson}
              </summary>
              <pre className="mt-2 overflow-x-auto rounded bg-rmb-light p-3 font-mono text-xs text-rmb-dark">
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
  if (!atoms.length) {
    return <p className="text-sm text-rmb-gray">{t.sessionDetail.noAtoms}</p>;
  }

  return (
    <div className="space-y-3">
      {atoms.map((atom) => (
        <article key={atom.id} className="rounded-lg border border-rmb-gray/15 bg-white p-4">
          <div className="flex flex-wrap items-center gap-2 text-xs text-rmb-gray">
            <span className="rounded bg-rmb-light px-2 py-0.5 font-medium text-rmb-dark">
              {atom.category}
            </span>
            {atom.scene_name && <span>{atom.scene_name}</span>}
            {atom.slug && <span className="font-mono">{atom.slug}</span>}
          </div>
          <p className="mt-2 text-sm text-rmb-dark">{atom.content}</p>
        </article>
      ))}
    </div>
  );
}

function ScenesTab({ scenes }: { scenes: SceneRow[] }) {
  const { t } = useI18n();
  if (!scenes.length) {
    return <p className="text-sm text-rmb-gray">{t.sessionDetail.noScenes}</p>;
  }

  return (
    <div className="space-y-3">
      {scenes.map((scene) => (
        <article key={scene.id} className="rounded-lg border border-rmb-gray/15 bg-white p-4">
          <h3 className="text-sm font-medium text-rmb-dark">
            {scene.display_name ?? scene.id}
          </h3>
          {scene.abstract && (
            <p className="mt-1 text-sm text-rmb-gray">{scene.abstract}</p>
          )}
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

  if (!sessionKey) {
    return <p className="text-rmb-gray">{t.sessionDetail.missingKey}</p>;
  }
  if (loading) return <p className="text-rmb-gray">{t.common.loading}</p>;
  if (error) return <p className="text-red-600">{error}</p>;
  if (!detail) return null;

  const { session, pipeline_state: pipeline, turns, atoms, scenes } = detail;
  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: "turns", label: t.sessionDetail.tabs.turns, count: turns.length },
    { id: "atoms", label: t.sessionDetail.tabs.atoms, count: atoms.length },
    { id: "scenes", label: t.sessionDetail.tabs.scenes, count: scenes.length },
  ];

  return (
    <div className="space-y-6">
      <div>
        <Link to="/sessions" className="text-sm text-rmb-accent hover:underline">
          ← {t.sessionDetail.back}
        </Link>
        <h1 className="mt-2 text-2xl font-semibold text-rmb-dark">{session.session_key}</h1>
        {session.abstract && (
          <p className="mt-2 text-sm text-rmb-gray">{session.abstract}</p>
        )}
        <div className="mt-3 flex flex-wrap gap-2">
          <StatusPill label="T1" value={pipeline?.t1_status ?? session.t1_status} />
          <StatusPill label="T2" value={pipeline?.t2_status ?? session.t2_status} />
          <StatusPill label="T3" value={pipeline?.t3_status ?? session.t3_status} />
          <StatusPill label={t.sessionDetail.turns} value={String(session.turn_count)} />
        </div>
      </div>

      <div className="flex gap-2 border-b border-rmb-gray/20">
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            onClick={() => setTab(item.id)}
            className={`border-b-2 px-3 py-2 text-sm font-medium transition ${
              tab === item.id
                ? "border-rmb-accent text-rmb-dark"
                : "border-transparent text-rmb-gray hover:text-rmb-dark"
            }`}
          >
            {item.label}
            <span className="ml-1.5 rounded bg-rmb-light px-1.5 py-0.5 text-xs tabular-nums text-rmb-gray">
              {item.count}
            </span>
          </button>
        ))}
      </div>

      {tab === "turns" && <TurnsTab turns={turns} />}
      {tab === "atoms" && <AtomsTab atoms={atoms} />}
      {tab === "scenes" && <ScenesTab scenes={scenes} />}
    </div>
  );
}
