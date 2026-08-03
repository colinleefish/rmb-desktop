import { useState } from "react";
import { Link } from "react-router-dom";
import { ExternalLink } from "lucide-react";
import { ConfigDiffReview } from "../agents/ConfigDiffReview";
import type { AgentSetupState } from "../../lib/agentSetupTypes";
import { applySetupArtifact } from "../../lib/setupApi";
import { useI18n } from "../../i18n";

export function AgentSetupPanel({
  agent,
  onAgentUpdated,
}: {
  agent: AgentSetupState;
  onAgentUpdated: (agent: AgentSetupState) => void;
}) {
  const { t } = useI18n();
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
  if (!hookArtifact || !recallArtifact) return null;
  const hookApplied = appliedIds.has(hookArtifact.id);
  const recallApplied = appliedIds.has(recallArtifact.id);

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

  return (
    <div className="space-y-6">
      {error && <p className="text-sm text-red-600">{error}</p>}
      <section className="space-y-4">
        <div>
          <h3 className="text-sm font-semibold text-rmb-dark">
            1. {t.agents.conversationCapture}
          </h3>
          <p className="mt-1 text-sm text-rmb-gray">{t.agents.conversationCaptureHint}</p>
        </div>
        <ConfigDiffReview
          artifact={hookArtifact}
          applied={hookApplied}
          applying={applyingId === hookArtifact.id}
          onApply={() => void applyArtifact(hookArtifact.id)}
        />
      </section>

      <section className="space-y-4">
        <div>
          <h3 className="text-sm font-semibold text-rmb-dark">
            2. {t.agents.recallInstructions}
          </h3>
          <p className="mt-1 text-sm text-rmb-gray">{t.agents.recallHint}</p>
        </div>
        <ConfigDiffReview
          artifact={recallArtifact}
          applied={recallApplied}
          applying={applyingId === recallArtifact.id}
          onApply={() => void applyArtifact(recallArtifact.id)}
        />
      </section>

      <section className="rounded-xl border border-rmb-gray/20 bg-rmb-light/30 p-5">
        <h3 className="text-sm font-semibold text-rmb-dark">3. {t.agents.verify}</h3>
        <p className="mt-2 text-sm text-rmb-gray">{t.agents.verifyHint}</p>
        <Link
          to={`/sessions?source=${
            agent.id === "claude-code" ? "cc" : agent.id === "codex" ? "codex" : agent.id
          }`}
          className="mt-4 inline-flex items-center gap-1.5 rounded-md border border-rmb-gray/20 bg-white px-3 py-2 text-sm font-medium text-rmb-dark hover:border-rmb-accent/40 hover:text-rmb-accent"
        >
          {t.agents.openSessions}
          <ExternalLink className="size-3.5" />
        </Link>
      </section>
    </div>
  );
}
