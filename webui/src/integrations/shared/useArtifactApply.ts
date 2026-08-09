import { useState } from "react";
import type { AgentSetupState } from "../../lib/agentSetupTypes";
import { applySetupArtifact } from "../../lib/setupApi";

export function useArtifactApply(
  agent: AgentSetupState,
  onAgentUpdated: (agent: AgentSetupState) => void,
) {
  const [appliedIds, setAppliedIds] = useState<Set<string>>(() => {
    const initial = new Set<string>();
    for (const artifact of agent.artifacts) {
      if (artifact.changeType === "unchanged" && artifact.applyMode === "write") {
        initial.add(artifact.id);
      }
    }
    return initial;
  });
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const hookArtifact = agent.artifacts[0];
  const recallArtifact = agent.artifacts[1];

  async function applyArtifact(id: string) {
    setApplyingId(id);
    setError(null);
    try {
      const updated = await applySetupArtifact(agent.id, id);
      onAgentUpdated(updated);
      setAppliedIds((prev) => new Set(prev).add(id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Apply failed");
    } finally {
      setApplyingId(null);
    }
  }

  return {
    hookArtifact,
    recallArtifact,
    hookApplied: hookArtifact ? appliedIds.has(hookArtifact.id) : false,
    recallApplied: recallArtifact ? appliedIds.has(recallArtifact.id) : false,
    applyingId,
    error,
    applyArtifact,
  };
}
