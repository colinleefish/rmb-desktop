import type { AgentSetupState } from "../../lib/agentSetupTypes";
import { agentPresenceStatus } from "../../lib/agentStatus";
import { useI18n } from "../../i18n";

export function AgentStatusIndicator({ agent }: { agent: AgentSetupState }) {
  const { t } = useI18n();
  const status = agentPresenceStatus(agent);

  if (status === "absent") {
    return (
      <span className="shrink-0 text-[10px] uppercase tracking-wide text-rmb-gray/40">
        {t.agents.notDetectedBadge}
      </span>
    );
  }

  const title =
    status === "configured"
      ? t.agents.configuredDotTitle
      : t.agents.unconfiguredDotTitle;

  const dotClass =
    status === "configured"
      ? "bg-emerald-500 ring-1 ring-emerald-500/30"
      : "bg-rmb-gray/35 ring-1 ring-rmb-gray/25";

  return (
    <span
      className={`size-2 shrink-0 rounded-full ${dotClass}`}
      title={title}
      aria-label={title}
    />
  );
}
