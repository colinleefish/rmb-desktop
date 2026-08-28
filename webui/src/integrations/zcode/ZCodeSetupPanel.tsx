import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";
import { useI18n } from "../../i18n";

export function ZCodeSetupPanel(props: IntegrationSetupPanelProps) {
  const { t } = useI18n();

  return (
    <HookRecallVerifySetup
      {...props}
      sessionSource="zcode"
      captureTitle={t.agents.zcode.captureTitle}
      captureHint={t.agents.zcode.captureHint}
    />
  );
}
