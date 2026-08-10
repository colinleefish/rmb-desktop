import { MOCK_AGENTS } from "./agentSetupMock";
import type { AgentSetupState } from "./agentSetupTypes";

const HOOK_ARTIFACTS = new Set(["hooks", "settings", "plugin", "extension"]);
const RECALL_ARTIFACTS = new Set(["user_rules", "claude_md", "agents_md"]);

let agents: AgentSetupState[] = structuredClone(MOCK_AGENTS);

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function hookStatusFor(agent: AgentSetupState): AgentSetupState["hookStatus"] {
  const hookArtifacts = agent.artifacts.filter((a) => HOOK_ARTIFACTS.has(a.id));
  if (hookArtifacts.length === 0) return agent.hookStatus;
  const allApplied = hookArtifacts.every((a) => a.current === a.proposed);
  return allApplied ? "configured" : "pending";
}

function recallStatusFor(agent: AgentSetupState): AgentSetupState["recallStatus"] {
  const recallArtifacts = agent.artifacts.filter((a) => RECALL_ARTIFACTS.has(a.id));
  if (recallArtifacts.length === 0) return agent.recallStatus;
  const allApplied = recallArtifacts.every(
    (a) => a.applyMode === "copy_only" || a.current === a.proposed,
  );
  return allApplied ? "configured" : "pending";
}

export async function fetchSetupStatus(): Promise<AgentSetupState[]> {
  await delay(150);
  return agents;
}

export async function fetchAgentPreview(agentId: string): Promise<AgentSetupState> {
  await delay(80);
  const agent = agents.find((a) => a.id === agentId);
  if (!agent) throw new Error(`Unknown agent: ${agentId}`);
  return agent;
}

export async function applySetupArtifact(
  agentId: string,
  artifactId: string,
): Promise<AgentSetupState> {
  await delay(250);
  agents = agents.map((agent) => {
    if (agent.id !== agentId) return agent;
    const artifacts = agent.artifacts.map((artifact) => {
      if (artifact.id !== artifactId) return artifact;
      if (artifact.applyMode === "copy_only") return artifact;
      return { ...artifact, current: artifact.proposed, changeType: "unchanged" as const };
    });
    const updated = { ...agent, artifacts };
    return {
      ...updated,
      hookStatus: hookStatusFor(updated),
      recallStatus: recallStatusFor(updated),
    };
  });
  const updated = agents.find((a) => a.id === agentId);
  if (!updated) throw new Error(`Unknown agent: ${agentId}`);
  return updated;
}
