import type { IntegrationAgentId } from "./agentRegistry";
import { isIntegrationAgentId } from "./agentRegistry";

export type SettingsSection =
  | "general"
  | "llm"
  | "embed"
  | "pipeline"
  | "integrations"
  | "language";

export function parseSettingsPath(pathname: string): {
  section: SettingsSection;
  agentId: IntegrationAgentId;
} {
  const rest = pathname.replace(/^\/settings\/?/, "");
  if (!rest) {
    return { section: "general", agentId: "cursor" };
  }
  if (rest === "integrations") {
    return { section: "integrations", agentId: "cursor" };
  }
  if (rest.startsWith("integrations/")) {
    const raw = rest.slice("integrations/".length);
    return {
      section: "integrations",
      agentId: isIntegrationAgentId(raw) ? raw : "cursor",
    };
  }
  const sections: SettingsSection[] = [
    "general",
    "llm",
    "embed",
    "pipeline",
    "language",
  ];
  if (sections.includes(rest as SettingsSection)) {
    return { section: rest as SettingsSection, agentId: "cursor" };
  }
  return { section: "general", agentId: "cursor" };
}

export function settingsPath(section: SettingsSection, agentId?: IntegrationAgentId): string {
  if (section === "integrations") {
    return `/settings/integrations/${agentId ?? "cursor"}`;
  }
  return `/settings/${section}`;
}
