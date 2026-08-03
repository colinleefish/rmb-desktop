import type { AgentSetupState } from "./agentSetupTypes";

/** Agent is on disk but rmb hooks / recall are not fully applied yet. */
export function isAgentConfigured(agent: AgentSetupState): boolean {
  if (!agent.detected) return false;

  const writable = agent.artifacts.filter((a) => a.applyMode === "write");
  if (writable.length > 0) {
    return writable.every((a) => a.changeType === "unchanged");
  }

  return agent.hookStatus === "configured" && agent.recallStatus === "configured";
}

export type AgentPresenceStatus = "absent" | "unconfigured" | "configured";

export function agentPresenceStatus(agent: AgentSetupState): AgentPresenceStatus {
  if (!agent.detected) return "absent";
  if (!isAgentConfigured(agent)) return "unconfigured";
  return "configured";
}
