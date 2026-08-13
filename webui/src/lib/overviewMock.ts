import type { Overview } from "./types";

/**
 * Story mirrors mockPipelineHealth(): ~100 imported sessions, most through T1,
 * fewer through T2, and a large T3 backlog. Object counts are sized so the
 * distillation pyramid reads as "lots of raw signal, fewer durable memories".
 */
export function mockOverview(): Overview {
  return {
    counts: {
      sessions: 100,
      turns: 2847,
      atoms: 1240,
      scenes: 386,
      memories: 48,
      pipeline_states: 100,
      tasks: 6,
      corrections: 7,
      skills: 15,
    },
    memory_by_category: {
      profile_version: 3,
      events: 18,
      preferences: 14,
      entities: 16,
    },
  };
}
