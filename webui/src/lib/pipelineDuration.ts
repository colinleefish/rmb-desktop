import type { ConfigView } from "./types";

export const PIPELINE_DURATION_FIELDS = [
  "l1_poll_interval",
  "l2_poll_interval",
  "l3_poll_interval",
  "embed_poll_interval",
  "l1_idle_seconds",
  "l2_delay_after_l1",
] as const satisfies ReadonlyArray<keyof ConfigView["pipeline"]>;

export type PipelineDurationField = (typeof PIPELINE_DURATION_FIELDS)[number];

/** Parse a Go duration string (e.g. "10m0s", "15s") into whole seconds. */
export function durationToSeconds(raw: string): number {
  const s = raw.trim();
  if (!s) return 0;
  if (/^\d+$/.test(s)) return Number(s);

  let totalMs = 0;
  const re = /(\d+(?:\.\d+)?)(h|m|s|ms)/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(s)) !== null) {
    const n = Number(match[1]);
    switch (match[2]) {
      case "h":
        totalMs += n * 3_600_000;
        break;
      case "m":
        totalMs += n * 60_000;
        break;
      case "s":
        totalMs += n * 1_000;
        break;
      case "ms":
        totalMs += n;
        break;
    }
  }
  return Math.round(totalMs / 1_000);
}

export function secondsToDuration(seconds: number): string {
  const n = Math.max(0, Math.round(seconds));
  return `${n}s`;
}
