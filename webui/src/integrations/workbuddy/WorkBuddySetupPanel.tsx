import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";
import { useI18n } from "../../i18n";

export function WorkBuddySetupPanel(props: IntegrationSetupPanelProps) {
  const { t } = useI18n();

  return (
    <HookRecallVerifySetup
      {...props}
      sessionSource="workbuddy"
      captureTitle={t.agents.workbuddy.captureTitle}
      captureHint={t.agents.workbuddy.captureHint}
    />
  );
}
