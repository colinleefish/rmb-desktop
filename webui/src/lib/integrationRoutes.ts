import type { IntegrationAgentId } from "./agentRegistry";
import { isIntegrationAgentId } from "./agentRegistry";

export function parseIntegrationPath(pathname: string): {
  agentId: IntegrationAgentId;
} {
  const rest = pathname.replace(/^\/integrations\/?/, "");
  if (!rest) {
    return { agentId: "cursor" };
  }
  return {
    agentId: isIntegrationAgentId(rest) ? rest : "cursor",
  };
}

export function integrationPath(agentId: IntegrationAgentId): string {
  return `/integrations/${agentId}`;
}
