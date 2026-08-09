import type { IntegrationDefinition } from "../types";
import { ClaudeCodeSetupPanel } from "./ClaudeCodeSetupPanel";
import logo from "./assets/logo.svg";

export { ClaudeCodeSetupPanel } from "./ClaudeCodeSetupPanel";

export const claudeCodeIntegration: IntegrationDefinition = {
  id: "claude-code",
  label: "Claude Code",
  logo,
  SetupPanel: ClaudeCodeSetupPanel,
};
