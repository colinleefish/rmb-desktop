export const MEMORY_CATEGORIES = [
  "profile",
  "events",
  "preferences",
  "entities",
] as const;

export type MemoryCategory = (typeof MEMORY_CATEGORIES)[number];

export function isMemoryCategory(value: string): value is MemoryCategory {
  return (MEMORY_CATEGORIES as readonly string[]).includes(value);
}

export const DEFAULT_MEMORY_CATEGORY: MemoryCategory = "profile";
