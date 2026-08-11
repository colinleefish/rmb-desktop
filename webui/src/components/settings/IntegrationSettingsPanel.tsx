import { useCallback, useEffect, useState } from "react";
import { AgentLogo } from "../../integrations/shared/AgentLogo";
import { AgentInactivePanel } from "../../integrations/shared/AgentInactivePanel";
import { AgentStatusIndicator } from "../../integrations/shared/AgentStatusIndicator";
import {
  getIntegration,
  INTEGRATIONS,
} from "../../integrations/registry";
import type { IntegrationAgentId } from "../../integrations/types";
import type { AgentSetupState } from "../../lib/agentSetupTypes";
import { fetchAgentPreview, fetchSetupStatus } from "../../lib/setupApi";
import { useI18n } from "../../i18n";

export function IntegrationSettingsPanel({
  agentId,
  onAgentChange,
  onAgentsChange,
}: {
  agentId: IntegrationAgentId;
  onAgentChange: (id: IntegrationAgentId) => void;
  onAgentsChange?: (agents: AgentSetupState[]) => void;
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
      setError(err instanceof Error ? err.message : "Failed to load agent setup");
    } finally {
      setLoading(false);
    }
  }, [agentId, onAgentsChange]);

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
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load agent"));
  }, [agentId, agents]);

  async function selectAgent(id: IntegrationAgentId) {
    onAgentChange(id);
    try {
      const preview = agents.find((a) => a.id === id) ?? (await fetchAgentPreview(id));
      setAgent(preview);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load agent");
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
    return <p className="text-sm text-red-600">{error}</p>;
  }

  if (!agent || !integration) return null;

  const { SetupPanel } = integration;
  const sidebarHeading =
    "px-2 pb-2 text-[11px] font-semibold uppercase tracking-wide text-rmb-gray/60";

  return (
    <div className="flex min-h-[32rem] gap-8">
      <aside className="w-48 shrink-0">
        <p className={sidebarHeading}>{t.agents.agentSidebarLabel}</p>
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
                  className={[
                    "flex w-full items-center gap-2.5 rounded-md px-2 py-2 text-left text-sm transition",
                    active && detected
                      ? "bg-rmb-accent/10 font-medium text-rmb-accent"
                      : active && !detected
                        ? "bg-rmb-gray/10 font-medium text-rmb-gray"
                        : detected
                          ? "text-rmb-gray hover:bg-rmb-light hover:text-rmb-dark"
                          : "text-rmb-gray/45 hover:bg-rmb-light/60 hover:text-rmb-gray/70",
                  ].join(" ")}
                >
                  <AgentLogo logo={entry.logo} inactive={!detected} size={18} />
                  <span className="min-w-0 flex-1 truncate">{entry.label}</span>
                  {meta && <AgentStatusIndicator agent={meta} />}
                </button>
              </li>
            );
          })}
        </ul>
      </aside>

      <div className="min-w-0 flex-1">
        {error && <p className="mb-4 text-sm text-red-600">{error}</p>}
        <p className={`${sidebarHeading} invisible`} aria-hidden>
          {t.agents.agentSidebarLabel}
        </p>
        <div className="flex items-center gap-3 px-2 py-2">
          <AgentLogo logo={integration.logo} inactive={!agent.detected} size={28} />
          <h2 className="text-lg font-semibold text-rmb-dark">{integration.label}</h2>
        </div>
        <div className="mt-6">
          {agent.detected ? (
            <SetupPanel agent={agent} onAgentUpdated={handleAgentUpdated} />
          ) : (
            <AgentInactivePanel agent={agent} />
          )}
        </div>
      </div>
    </div>
  );
}
