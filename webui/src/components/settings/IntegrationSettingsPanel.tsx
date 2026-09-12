import { useCallback, useEffect, useState } from "react";
import { Info } from "lucide-react";
import { AgentLogo } from "../../integrations/shared/AgentLogo";
import { AgentInactivePanel } from "../../integrations/shared/AgentInactivePanel";
import { AgentStatusIndicator } from "../../integrations/shared/AgentStatusIndicator";
import {
  getIntegration,
  INTEGRATIONS,
} from "../../integrations/registry";
import type { IntegrationAgentId } from "../../integrations/types";
import type { AgentSetupState } from "../../lib/agentSetupTypes";
import { fetchAgentPreview, fetchSetupStatus, isSetupMocked } from "../../lib/setupApi";
import { agentSetupStatusCopy } from "../../lib/agentStatusCopy";
import { useI18n } from "../../i18n";
import { PageHeader } from "../PageHeader";
import { StatusPill } from "../StatusPill";

function switcherItemClass(active: boolean, detected: boolean): string {
  return [
    "flex h-8 w-full items-center gap-2.5 rounded-md px-2.5 text-left text-sm transition-colors",
    active
      ? "bg-rmb-fill font-medium text-rmb-dark"
      : detected
        ? "text-rmb-muted hover:bg-rmb-fill hover:text-rmb-dark"
        : "text-rmb-faint hover:bg-rmb-fill hover:text-rmb-dark",
  ].join(" ");
}

export function IntegrationSettingsPanel({
  agentId,
  onAgentChange,
  onAgentsChange,
  showAgentSwitcher = true,
}: {
  agentId: IntegrationAgentId;
  onAgentChange?: (id: IntegrationAgentId) => void;
  onAgentsChange?: (agents: AgentSetupState[]) => void;
  /** When false, single-agent focus (Agents hub detail); onboarding keeps the switcher. */
  showAgentSwitcher?: boolean;
}) {
  const { t } = useI18n();
  const [agents, setAgents] = useState<AgentSetupState[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [agent, setAgent] = useState<AgentSetupState | null>(null);

  const integration = getIntegration(agentId);

  const loadAgents = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await fetchSetupStatus();
      setAgents(list);
      onAgentsChange?.(list);
      const preview =
        list.find((a) => a.id === agentId) ?? (await fetchAgentPreview(agentId));
      setAgent(preview);
    } catch (err) {
      setError(err instanceof Error ? err.message : t.agents.loadSetupError);
    } finally {
      setLoading(false);
    }
  }, [agentId, onAgentsChange, t]);

  useEffect(() => {
    void loadAgents();
  }, [loadAgents]);

  useEffect(() => {
    const cached = agents.find((a) => a.id === agentId);
    if (cached) {
      setAgent(cached);
      return;
    }
    void fetchAgentPreview(agentId)
      .then(setAgent)
      .catch((err) => setError(err instanceof Error ? err.message : t.agents.loadAgentError));
  }, [agentId, agents, t]);

  async function selectAgent(id: IntegrationAgentId) {
    onAgentChange?.(id);
    try {
      const preview = agents.find((a) => a.id === id) ?? (await fetchAgentPreview(id));
      setAgent(preview);
    } catch (err) {
      setError(err instanceof Error ? err.message : t.agents.loadAgentError);
    }
  }

  function handleAgentUpdated(updated: AgentSetupState) {
    setAgent(updated);
    setAgents((prev) => {
      const next = prev.map((a) => (a.id === updated.id ? updated : a));
      onAgentsChange?.(next);
      return next;
    });
  }

  if (loading && !agent) {
    return <p className="text-sm text-rmb-gray">{t.common.loading}</p>;
  }

  if (error && !agent) {
    return <p className="text-sm text-rmb-danger">{error}</p>;
  }

  if (!agent || !integration) return null;

  const { SetupPanel } = integration;
  const copy = agentSetupStatusCopy(agent, t);

  return (
    <div className="space-y-4">
      <PageHeader
        back={{ to: "/agents", label: t.agents.backToHub }}
        icon={<AgentLogo logo={integration.logo} inactive={!agent.detected} size={22} />}
        title={integration.label}
        meta={<StatusPill tone={copy.tone}>{copy.label}</StatusPill>}
      />

      {isSetupMocked() && (
        <p className="inline-flex items-center gap-1.5 rounded-md border border-rmb-line bg-rmb-warn-soft px-2.5 py-1.5 text-xs text-rmb-warn">
          <Info className="size-3.5 shrink-0" aria-hidden />
          {t.agents.previewBanner}
        </p>
      )}

      <div className="overflow-hidden rounded-md border border-rmb-line bg-white">
        <div className={showAgentSwitcher ? "flex min-h-[28rem]" : "min-h-[28rem]"}>
          {showAgentSwitcher && (
            <aside className="w-48 shrink-0 border-r border-rmb-line p-3">
              <ul className="space-y-0.5">
                {INTEGRATIONS.map((entry) => {
                  const meta = agents.find((a) => a.id === entry.id);
                  const active = agentId === entry.id;
                  const detected = meta?.detected ?? false;
                  return (
                    <li key={entry.id}>
                      <button
                        type="button"
                        onClick={() => void selectAgent(entry.id)}
                        className={switcherItemClass(active, detected)}
                      >
                        <AgentLogo logo={entry.logo} inactive={!detected} size={16} />
                        <span className="min-w-0 flex-1 truncate">{entry.label}</span>
                        {meta && <AgentStatusIndicator agent={meta} />}
                      </button>
                    </li>
                  );
                })}
              </ul>
            </aside>
          )}

          <div className="min-w-0 flex-1 p-6">
            {error && <p className="mb-4 text-sm text-rmb-danger">{error}</p>}
            {agent.detected ? (
              <SetupPanel agent={agent} onAgentUpdated={handleAgentUpdated} />
            ) : (
              <AgentInactivePanel agent={agent} />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
