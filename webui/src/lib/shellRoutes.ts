import type { Translation } from "../i18n/translations";
import { isMemoryCategory } from "./memoryCategories";

export function getMemoriesSectionMeta(
  pathname: string,
  t: Translation,
): { title: string; subtitle: string } | null {
  const match = pathname.match(/^\/memories(?:\/([^/]+))?\/?$/);
  if (!match) return null;

  const categoryParam = match[1];
  if (!categoryParam) {
    return { title: t.memories.tabAll, subtitle: t.memories.subtitle };
  }
  if (isMemoryCategory(categoryParam)) {
    return {
      title: t.memories.categories[categoryParam].title,
      subtitle: t.memories.categories[categoryParam].subtitle,
    };
  }
  return null;
}

export function getRouteMeta(
  pathname: string,
  t: Translation,
): { title: string; subtitle: string } {
  if (pathname === "/" || pathname === "") {
    return { title: t.overview.title, subtitle: t.shell.overviewSubtitle };
  }
  if (pathname.startsWith("/sessions")) {
    return { title: t.sessions.title, subtitle: t.shell.sessionsSubtitle };
  }
  if (pathname.startsWith("/memories")) {
    return { title: t.shell.memoriesTitle, subtitle: t.shell.memoriesSubtitle };
  }
  if (pathname.startsWith("/agents/skills") || pathname.startsWith("/skills")) {
    return { title: t.skills.title, subtitle: t.shell.skillsSubtitle };
  }
  if (pathname.startsWith("/agents") || pathname.startsWith("/integrations")) {
    return { title: t.agents.hubTitle, subtitle: t.shell.agentsSubtitle };
  }
  if (pathname.startsWith("/settings")) {
    return { title: t.settings.title, subtitle: t.shell.settingsSubtitle };
  }
  return { title: t.appName, subtitle: "" };
}
