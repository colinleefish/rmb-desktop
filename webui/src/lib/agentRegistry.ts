export type IntegrationAgentId =
  | "cursor"
  | "claude-code"
  | "codex"
  | "opencode"
  | "pi";

export type AgentRegistryEntry = {
  id: IntegrationAgentId;
  label: string;
  logo: string;
};

export const AGENT_REGISTRY: AgentRegistryEntry[] = [
  { id: "cursor", label: "Cursor", logo: "cursor.svg" },
  { id: "claude-code", label: "Claude Code", logo: "claude-code.svg" },
  { id: "codex", label: "Codex", logo: "codex.svg" },
  { id: "opencode", label: "OpenCode", logo: "opencode.svg" },
  { id: "pi", label: "Pi", logo: "pi.png" },
];

export function isIntegrationAgentId(value: string | null): value is IntegrationAgentId {
  return AGENT_REGISTRY.some((a) => a.id === value);
}
