import { useCallback, useEffect, useState } from "react";
import { AgentLogo } from "../agents/AgentLogo";
import { AgentInactivePanel } from "../agents/AgentInactivePanel";
import { AgentSetupPanel } from "../agents/AgentSetupPanel";
import { AgentStatusIndicator } from "../agents/AgentStatusIndicator";
import { AGENT_REGISTRY, type IntegrationAgentId } from "../../lib/agentRegistry";
import type { AgentSetupState } from "../../lib/agentSetupTypes";
import { fetchAgentPreview, fetchSetupStatus } from "../../lib/setupApi";
import { useI18n } from "../../i18n";

export function IntegrationSettingsPanel({
  agentId,
  onAgentChange,
}: {
  agentId: IntegrationAgentId;
  onAgentChange: (id: IntegrationAgentId) => void;
}) {
  const { t } = useI18n();
  const [agents, setAgents] = useState<AgentSetupState[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [agent, setAgent] = useState<AgentSetupState | null>(null);

  const loadAgents = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await fetchSetupStatus();
      setAgents(list);
      const preview =
        list.find((a) => a.id === agentId) ?? (await fetchAgentPreview(agentId));
      setAgent(preview);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load agent setup");
    } finally {
      setLoading(false);
    }
  }, [agentId]);

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

  const registry = AGENT_REGISTRY.find((a) => a.id === agentId);

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
    setAgents((prev) => prev.map((a) => (a.id === updated.id ? updated : a)));
  }

  if (loading && !agent) {
    return <p className="text-sm text-rmb-gray">{t.common.loading}</p>;
  }

  if (error && !agent) {
    return <p className="text-sm text-red-600">{error}</p>;
  }

  if (!agent || !registry) return null;

  return (
    <div className="flex min-h-[32rem] gap-8">
      <aside className="w-48 shrink-0">
        <p className="px-2 pb-2 text-[11px] font-semibold uppercase tracking-wide text-rmb-gray/60">
          {t.agents.agentSidebarLabel}
        </p>
        <ul className="space-y-0.5">
          {AGENT_REGISTRY.map((entry) => {
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
                  <AgentLogo agent={entry} inactive={!detected} size={18} />
                  <span className="min-w-0 flex-1 truncate">{entry.label}</span>
                  {meta && <AgentStatusIndicator agent={meta} />}
                </button>
              </li>
            );
          })}
        </ul>
      </aside>

      <div className="min-w-0 flex-1 space-y-6">
        {error && <p className="text-sm text-red-600">{error}</p>}
        <div className="flex items-center gap-3">
          <AgentLogo agent={registry} inactive={!agent.detected} size={28} />
          <h2 className="text-lg font-semibold text-rmb-dark">{registry.label}</h2>
        </div>
        {agent.detected ? (
          <AgentSetupPanel agent={agent} onAgentUpdated={handleAgentUpdated} />
        ) : (
          <AgentInactivePanel agent={agent} />
        )}
      </div>
    </div>
  );
}
