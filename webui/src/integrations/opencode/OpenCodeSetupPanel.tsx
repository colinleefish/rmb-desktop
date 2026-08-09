import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";

export function OpenCodeSetupPanel(props: IntegrationSetupPanelProps) {
  return <HookRecallVerifySetup {...props} sessionSource="opencode" />;
}
