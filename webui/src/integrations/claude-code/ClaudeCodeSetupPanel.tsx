import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";

export function ClaudeCodeSetupPanel(props: IntegrationSetupPanelProps) {
  return <HookRecallVerifySetup {...props} sessionSource="cc" />;
}
