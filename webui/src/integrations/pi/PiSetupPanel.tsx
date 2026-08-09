import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";

export function PiSetupPanel(props: IntegrationSetupPanelProps) {
  return <HookRecallVerifySetup {...props} sessionSource="pi" />;
}
