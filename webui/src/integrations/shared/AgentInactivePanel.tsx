import { MonitorOff } from "lucide-react";
import type { AgentSetupState } from "../../lib/agentSetupTypes";
import { useI18n } from "../../i18n";

export function AgentInactivePanel({ agent }: { agent: AgentSetupState }) {
  const { t } = useI18n();

  return (
    <p className="flex items-center gap-2 text-sm text-rmb-gray/70">
      <MonitorOff className="size-4 shrink-0 stroke-rmb-gray/50" />
      {t.agents.notDetectedLine.replace("{name}", agent.name)}
    </p>
  );
}
