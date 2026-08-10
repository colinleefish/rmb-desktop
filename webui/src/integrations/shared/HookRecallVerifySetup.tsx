import { Link } from "react-router-dom";
import { ExternalLink } from "lucide-react";
import type { ReactNode } from "react";
import type { IntegrationSetupPanelProps } from "../types";
import type { SetupArtifact } from "../../lib/agentSetupTypes";
import { useI18n } from "../../i18n";
import { ConfigDiffReview } from "./ConfigDiffReview";
import { FileReplaceReview } from "./FileReplaceReview";
import { RecallRuleCopy } from "./RecallRuleCopy";
import { SetupGuideStep } from "./SetupGuideStep";
import { useArtifactApply } from "./useArtifactApply";

export type RecallStepRenderProps = {
  artifact: SetupArtifact;
  applied: boolean;
  applying: boolean;
  onApply: () => void;
};

export function HookRecallVerifySetup({
  agent,
  onAgentUpdated,
  sessionSource,
  renderRecall,
  captureHint,
  captureTitle,
}: IntegrationSetupPanelProps & {
  sessionSource: string;
  renderRecall?: (props: RecallStepRenderProps) => ReactNode;
  captureHint?: string;
  captureTitle?: string;
}) {
  const { t } = useI18n();
  const {
    hookArtifact,
    recallArtifact,
    hookApplied,
    recallApplied,
    applyingId,
    error,
    applyArtifact,
  } = useArtifactApply(agent, onAgentUpdated);

  if (!hookArtifact || !recallArtifact) {
    return (
      <p className="text-sm text-rmb-gray">{t.agents.setupIncompleteHint}</p>
    );
  }

  function defaultRecallStep(props: RecallStepRenderProps) {
    if (props.artifact.applyMode === "copy_only") {
      return <RecallRuleCopy artifact={props.artifact} />;
    }
    return (
      <ConfigDiffReview
        artifact={props.artifact}
        title={t.agents.recallInstructions}
        applied={props.applied}
        applying={props.applying}
        onApply={props.onApply}
      />
    );
  }

  const recallRenderer = renderRecall ?? defaultRecallStep;

  const CaptureReview =
    hookArtifact.displayMode === "replace" ? FileReplaceReview : ConfigDiffReview;

  return (
    <div className="relative ml-2 pl-10">
      {error && <p className="mb-6 text-sm text-red-600">{error}</p>}

      <SetupGuideStep step={1}>
        <div className="space-y-4">
          <div>
            <h3 className="text-sm font-semibold text-rmb-dark">
              {captureTitle ?? t.agents.conversationCapture}
            </h3>
            <p className="mt-1 text-sm text-rmb-gray">
              {captureHint ?? t.agents.conversationCaptureHint}
            </p>
          </div>
          <CaptureReview
            artifact={hookArtifact}
            title={captureTitle ?? t.agents.conversationCapture}
            applied={hookApplied}
            applying={applyingId === hookArtifact.id}
            onApply={() => void applyArtifact(hookArtifact.id)}
          />
        </div>
      </SetupGuideStep>

      <SetupGuideStep step={2}>
        <div className="space-y-4">
          <div>
            <h3 className="text-sm font-semibold text-rmb-dark">
              {t.agents.recallInstructions}
            </h3>
            <p className="mt-1 text-sm text-rmb-gray">{t.agents.recallHint}</p>
          </div>
          {recallRenderer({
            artifact: recallArtifact,
            applied: recallApplied,
            applying: applyingId === recallArtifact.id,
            onApply: () => void applyArtifact(recallArtifact.id),
          })}
        </div>
      </SetupGuideStep>

      <SetupGuideStep step={3} isLast>
        <div className="space-y-4">
          <div>
            <h3 className="text-sm font-semibold text-rmb-dark">{t.agents.verify}</h3>
            <p className="mt-1 text-sm text-rmb-gray">{t.agents.verifyHint}</p>
          </div>
          <Link
            to={`/sessions?source=${sessionSource}`}
            className="inline-flex items-center gap-1.5 rounded-md border border-rmb-gray/20 bg-white px-3 py-2 text-sm font-medium text-rmb-dark hover:border-rmb-accent/40 hover:text-rmb-accent"
          >
            {t.agents.openSessions}
            <ExternalLink className="size-3.5" />
          </Link>
        </div>
      </SetupGuideStep>
    </div>
  );
}
