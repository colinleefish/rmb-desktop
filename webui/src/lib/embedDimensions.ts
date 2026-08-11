export const EMBED_DIMENSION_OPTIONS = [384, 768, 1024, 1536] as const;

export type EmbedDimension = (typeof EMBED_DIMENSION_OPTIONS)[number];

export const DEFAULT_EMBED_DIMENSION: EmbedDimension = 1024;

export function isEmbedDimension(value: number): value is EmbedDimension {
  return (EMBED_DIMENSION_OPTIONS as readonly number[]).includes(value);
}

export function normalizeEmbedDimension(value: number): EmbedDimension {
  if (isEmbedDimension(value)) return value;
  return DEFAULT_EMBED_DIMENSION;
}
