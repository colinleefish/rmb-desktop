import type { IntegrationSetupPanelProps } from "../types";
import { HookRecallVerifySetup } from "../shared/HookRecallVerifySetup";

export function CodexSetupPanel(props: IntegrationSetupPanelProps) {
  return <HookRecallVerifySetup {...props} sessionSource="codex" />;
}
