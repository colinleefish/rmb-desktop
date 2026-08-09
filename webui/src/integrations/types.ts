import type { ComponentType } from "react";
import type { AgentSetupState } from "../lib/agentSetupTypes";

export type IntegrationAgentId =
  | "cursor"
  | "claude-code"
  | "codex"
  | "opencode"
  | "pi";

export type IntegrationSetupPanelProps = {
  agent: AgentSetupState;
  onAgentUpdated: (agent: AgentSetupState) => void;
};

export type IntegrationDefinition = {
  id: IntegrationAgentId;
  label: string;
  logo: string;
  SetupPanel: ComponentType<IntegrationSetupPanelProps>;
};
