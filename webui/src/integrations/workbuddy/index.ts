import type { IntegrationDefinition } from "../types";
import { WorkBuddySetupPanel } from "./WorkBuddySetupPanel";
import logo from "./assets/logo.png";

export { WorkBuddySetupPanel } from "./WorkBuddySetupPanel";

export const workbuddyIntegration: IntegrationDefinition = {
  id: "workbuddy",
  label: "WorkBuddy",
  logo,
  SetupPanel: WorkBuddySetupPanel,
};
