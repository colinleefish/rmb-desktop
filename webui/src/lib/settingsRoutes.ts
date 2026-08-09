export type SettingsSection = "general" | "models" | "advanced";

export function parseSettingsPath(pathname: string): { section: SettingsSection } {
  const rest = pathname.replace(/^\/settings\/?/, "");
  if (!rest) {
    return { section: "general" };
  }
  if (rest === "llm" || rest === "embed") {
    return { section: "models" };
  }
  if (rest === "pipeline") {
    return { section: "advanced" };
  }
  if (rest === "language") {
    return { section: "general" };
  }
  const sections: SettingsSection[] = ["general", "models", "advanced"];
  if (sections.includes(rest as SettingsSection)) {
    return { section: rest as SettingsSection };
  }
  return { section: "general" };
}

export function settingsPath(section: SettingsSection): string {
  return `/settings/${section}`;
}
