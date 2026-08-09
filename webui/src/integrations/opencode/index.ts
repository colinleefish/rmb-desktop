import type { IntegrationDefinition } from "../types";
import { OpenCodeSetupPanel } from "./OpenCodeSetupPanel";
import logo from "./assets/logo.svg";

export { OpenCodeSetupPanel } from "./OpenCodeSetupPanel";

export const opencodeIntegration: IntegrationDefinition = {
  id: "opencode",
  label: "OpenCode",
  logo,
  SetupPanel: OpenCodeSetupPanel,
};
