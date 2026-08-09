export type { IntegrationAgentId } from "../integrations/types";
export {
  INTEGRATIONS,
  getIntegration,
  isIntegrationAgentId,
} from "../integrations/registry";

import { INTEGRATIONS } from "../integrations/registry";

/** @deprecated Use INTEGRATIONS from integrations/registry */
export const AGENT_REGISTRY = INTEGRATIONS.map(({ id, label, logo }) => ({
  id,
  label,
  logo,
}));

export type AgentRegistryEntry = (typeof AGENT_REGISTRY)[number];
