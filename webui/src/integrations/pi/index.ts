import type { IntegrationDefinition } from "../types";
import { PiSetupPanel } from "./PiSetupPanel";
import logo from "./assets/logo.png";

export { PiSetupPanel } from "./PiSetupPanel";

export const piIntegration: IntegrationDefinition = {
  id: "pi",
  label: "Pi",
  logo,
  SetupPanel: PiSetupPanel,
};
