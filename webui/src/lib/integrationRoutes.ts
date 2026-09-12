import type { IntegrationAgentId } from "./agentRegistry";
import { isIntegrationAgentId } from "./agentRegistry";

export function parseIntegrationPath(pathname: string): {
  agentId: IntegrationAgentId;
} {
  const rest = pathname.replace(/^\/(?:agents|integrations)\/?/, "");
  const id = rest.split("/")[0] ?? "";
  if (!id || id === "skills") {
    return { agentId: "cursor" };
  }
  return {
    agentId: isIntegrationAgentId(id) ? id : "cursor",
  };
}

export function integrationPath(agentId: IntegrationAgentId): string {
  return `/agents/${agentId}`;
}
