import type { Lang } from "./translations";

/** Display labels use each language's native name — add entries here when adding locales. */
export const SUPPORTED_LANGUAGES: ReadonlyArray<{ id: Lang; nativeLabel: string }> = [
  { id: "en", nativeLabel: "English" },
  { id: "zh", nativeLabel: "中文" },
];

export function isLang(value: string): value is Lang {
  return SUPPORTED_LANGUAGES.some((entry) => entry.id === value);
}

export const DEFAULT_LANG: Lang = "en";

/** Match browser language preference (Chrome-style: checks navigator.languages first). */
export function detectBrowserLang(): Lang {
  const candidates =
    navigator.languages?.length > 0 ? navigator.languages : [navigator.language];
  for (const raw of candidates) {
    const code = raw.toLowerCase();
    if (code.startsWith("zh")) return "zh";
    if (code.startsWith("en")) return "en";
  }
  return DEFAULT_LANG;
}
