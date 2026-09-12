import type { PillTone } from "../components/StatusPill";
import type { Translation } from "../i18n/translations";
import { agentPresenceStatus, type AgentPresenceStatus } from "./agentStatus";
import type { AgentSetupState } from "./agentSetupTypes";

export function agentPresenceCopy(
  status: AgentPresenceStatus,
  t: Translation,
): { label: string; cta: string; tone: PillTone } {
  if (status === "configured") {
    return { label: t.agents.statusConnected, cta: t.agents.details, tone: "ok" };
  }
  if (status === "unconfigured") {
    return { label: t.agents.statusUnconfigured, cta: t.agents.fixSetup, tone: "attention" };
  }
  return { label: t.agents.statusAbsent, cta: t.agents.setUp, tone: "neutral" };
}

export function agentSetupStatusCopy(
  agent: AgentSetupState,
  t: Translation,
): { label: string; tone: PillTone } {
  const { label, tone } = agentPresenceCopy(agentPresenceStatus(agent), t);
  return { label, tone };
}
