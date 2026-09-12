import { useEffect, useState } from "react";
import { Link, Navigate, useLocation, useParams } from "react-router-dom";
import { IntegrationSettingsPanel } from "../components/settings/IntegrationSettingsPanel";
import { EmptyState, ErrorNote } from "../components/EmptyState";
import { tabClass } from "../lib/tabClass";
import { StatusPill } from "../components/StatusPill";
import { AgentLogo } from "../integrations/shared/AgentLogo";
import { INTEGRATIONS } from "../integrations/registry";
import type { IntegrationAgentId } from "../integrations/types";
import { agentPresenceStatus } from "../lib/agentStatus";
import { agentPresenceCopy } from "../lib/agentStatusCopy";
import { isIntegrationAgentId } from "../lib/agentRegistry";
import { integrationPath } from "../lib/integrationRoutes";
import { fetchSetupStatus } from "../lib/setupApi";
import type { AgentSetupState } from "../lib/agentSetupTypes";
import { formatDateTime, formatDateTimeMonoClass } from "../lib/format";
import { SkillsPage } from "./SkillsPage";
import { useI18n } from "../i18n";

function AgentsTabs({ skills }: { skills: boolean }) {
  const { t } = useI18n();
  return (
    <div className="flex gap-1 border-b border-rmb-line">
      <Link to="/agents" className={tabClass(!skills)}>
        {t.agents.tabIntegration}
      </Link>
      <Link to="/agents/skills" className={tabClass(skills)}>
        {t.agents.tabSkills}
      </Link>
    </div>
  );
}

function agentCardSubtitle(
  status: ReturnType<typeof agentPresenceStatus>,
  agent: AgentSetupState | undefined,
  agentLabel: string,
  t: ReturnType<typeof useI18n>["t"],
): string | null {
  if (status === "absent") {
    return t.agents.notDetectedLine.replace("{name}", agentLabel);
  }
  if (agent?.lastHookAt) return null;
  if (agent?.hookStatus === "configured") return t.agents.noCaptureYet;
  return t.agents.noHook;
}

function AgentCardSkeleton() {
  return (
    <div aria-hidden className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="h-32 rounded-md border border-rmb-line p-4">
          <div className="h-7 w-7 rounded bg-rmb-fill" />
          <div className="mt-4 h-3 w-24 rounded bg-rmb-fill" />
          <div className="mt-2 h-3 w-32 rounded bg-rmb-fill" />
        </div>
      ))}
    </div>
  );
}

function AgentGrid() {
  const { t } = useI18n();
  const [byId, setById] = useState<Map<string, AgentSetupState>>(new Map());
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchSetupStatus()
      .then((list) => setById(new Map(list.map((a) => [a.id, a]))))
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <AgentCardSkeleton />;
  if (error) return <ErrorNote>{error}</ErrorNote>;
  if (INTEGRATIONS.length === 0) return <EmptyState title={t.agents.statusAbsent} />;

  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {INTEGRATIONS.map((entry) => {
        const agent = byId.get(entry.id);
        const status = agent ? agentPresenceStatus(agent) : "absent";
        const copy = agentPresenceCopy(status, t);
        const detected = agent?.detected ?? false;
        const lineText = agentCardSubtitle(status, agent, entry.label, t);
        return (
          <Link
            key={entry.id}
            to={integrationPath(entry.id)}
            className={[
              "group flex min-h-32 flex-col rounded-md border p-4 transition-colors",
              status === "unconfigured"
                ? "border-rmb-warn/40 hover:border-rmb-warn/70"
                : "border-rmb-line hover:border-rmb-line-strong",
            ].join(" ")}
          >
            <div className="flex items-center gap-3">
              <AgentLogo logo={entry.logo} size={28} inactive={!detected} />
              <span
                className={`min-w-0 flex-1 truncate text-sm font-semibold ${
                  detected ? "text-rmb-dark" : "text-rmb-muted"
                }`}
              >
                {entry.label}
              </span>
              <StatusPill tone={copy.tone}>{copy.label}</StatusPill>
            </div>
            <p className="mt-3 text-xs text-rmb-muted">
              {lineText !== null ? (
                <span className="truncate">{lineText}</span>
              ) : agent?.lastHookAt ? (
                <span className="inline-flex max-w-full flex-wrap items-baseline gap-x-1">
                  <span>{t.agents.lastCapture} ·</span>
                  <span className={formatDateTimeMonoClass}>{formatDateTime(agent.lastHookAt)}</span>
                </span>
              ) : null}
            </p>
            <span
              className={`mt-auto pt-3 text-xs font-medium ${
                status === "unconfigured" ? "text-rmb-warn" : "text-rmb-accent"
              }`}
            >
              {copy.cta}
            </span>
          </Link>
        );
      })}
    </div>
  );
}

export function AgentsPage() {
  const location = useLocation();
  const { agentId: agentParam } = useParams<{ agentId?: string }>();
  const skillsTab = location.pathname.includes("/skills");

  if (agentParam && !isIntegrationAgentId(agentParam)) {
    return <Navigate to="/agents" replace />;
  }

  if (agentParam) {
    const agentId = agentParam as IntegrationAgentId;
    return (
      <IntegrationSettingsPanel agentId={agentId} showAgentSwitcher={false} />
    );
  }

  return (
    <div className="space-y-5">
      <AgentsTabs skills={skillsTab} />
      {skillsTab ? <SkillsPage embedded /> : <AgentGrid />}
    </div>
  );
}
