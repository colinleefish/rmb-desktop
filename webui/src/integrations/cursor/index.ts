import type { IntegrationDefinition } from "../types";
import { CursorSetupPanel } from "./CursorSetupPanel";
import logo from "./assets/logo.svg";

export { CursorSetupPanel } from "./CursorSetupPanel";

export const cursorIntegration: IntegrationDefinition = {
  id: "cursor",
  label: "Cursor",
  logo,
  SetupPanel: CursorSetupPanel,
};
