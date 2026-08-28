import type { IntegrationDefinition } from "../types";
import { ZCodeSetupPanel } from "./ZCodeSetupPanel";
import logo from "./assets/logo.svg";

export { ZCodeSetupPanel } from "./ZCodeSetupPanel";

export const zcodeIntegration: IntegrationDefinition = {
  id: "zcode",
  label: "ZCode",
  logo,
  SetupPanel: ZCodeSetupPanel,
};
