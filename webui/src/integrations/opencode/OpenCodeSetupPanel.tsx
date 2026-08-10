import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";
import { useI18n } from "../../i18n";

export function OpenCodeSetupPanel(props: IntegrationSetupPanelProps) {
  const { t } = useI18n();

  return (
    <HookRecallVerifySetup
      {...props}
      sessionSource="opencode"
      captureTitle={t.agents.opencode.captureTitle}
      captureHint={t.agents.opencode.captureHint}
    />
  );
}
