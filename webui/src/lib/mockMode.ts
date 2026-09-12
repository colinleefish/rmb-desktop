/**
 * Dev-only global mock mode: every /api fetch is served by the in-memory mock
 * router (lib/mock/server.ts) instead of rmbd, so the SPA can run with no
 * daemon. Enable with `npm run dev:mock` (VITE_MOCK=true) or, inside any dev
 * session, by setting localStorage `rmb.mock = "1"` (the corner MOCK badge
 * toggles it). Per-feature flags (setup/pipeline/onboarding) honor this too.
 */
export function isFullMock(): boolean {
  if (import.meta.env.VITE_MOCK === "true") return true;
  if (!import.meta.env.DEV) return false;
  try {
    return localStorage.getItem("rmb.mock") === "1";
  } catch {
    return false;
  }
}

/** True when the env var (not localStorage alone) forces mock mode on. */
export function isMockEnvForced(): boolean {
  return import.meta.env.VITE_MOCK === "true";
}
