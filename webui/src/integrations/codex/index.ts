import type { IntegrationDefinition } from "../types";
import { CodexSetupPanel } from "./CodexSetupPanel";
import logo from "./assets/logo.svg";

export { CodexSetupPanel } from "./CodexSetupPanel";

export const codexIntegration: IntegrationDefinition = {
  id: "codex",
  label: "Codex",
  logo,
  SetupPanel: CodexSetupPanel,
};
