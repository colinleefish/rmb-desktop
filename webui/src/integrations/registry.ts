import type { IntegrationAgentId, IntegrationDefinition } from "./types";
import { cursorIntegration } from "./cursor";
import { claudeCodeIntegration } from "./claude-code";
import { codexIntegration } from "./codex";
import { opencodeIntegration } from "./opencode";
import { piIntegration } from "./pi";
import { workbuddyIntegration } from "./workbuddy";
import { zcodeIntegration } from "./zcode";

export const INTEGRATIONS: IntegrationDefinition[] = [
  cursorIntegration,
  claudeCodeIntegration,
  codexIntegration,
  opencodeIntegration,
  piIntegration,
  workbuddyIntegration,
  zcodeIntegration,
];

export function getIntegration(id: IntegrationAgentId): IntegrationDefinition | undefined {
  return INTEGRATIONS.find((entry) => entry.id === id);
}

export function isIntegrationAgentId(value: string | null): value is IntegrationAgentId {
  return INTEGRATIONS.some((entry) => entry.id === value);
}
