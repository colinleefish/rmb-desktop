import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";
import { useI18n } from "../../i18n";

export function PiSetupPanel(props: IntegrationSetupPanelProps) {
  const { t } = useI18n();

  return (
    <HookRecallVerifySetup
      {...props}
      sessionSource="pi"
      captureTitle={t.agents.pi.captureTitle}
      captureHint={t.agents.pi.captureHint}
    />
  );
}
