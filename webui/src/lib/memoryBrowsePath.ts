import { isMemoryCategory } from "./memoryCategories";

/** Maps rmb://memories/{category}/{slug} to a memories tab + search for that slug. */
export function memoryBrowsePath(memoryURI: string): string | null {
  const match = /^rmb:\/\/memories\/([^/]+)\/([^/?#]+)/.exec(memoryURI.trim());
  if (!match) return null;
  const category = match[1];
  const slug = match[2];
  if (!isMemoryCategory(category)) return null;
  return `/memories/${category}?q=${encodeURIComponent(slug)}`;
}
