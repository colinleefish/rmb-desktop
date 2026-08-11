import { clearOnboardingState } from "./onboardingState";
import { isOnboardingDemo } from "./onboardingMock";

const API = "/api/v1/onboarding";
const COMPLETE_KEY = "rmb.onboardingComplete";

type OnboardingStatus = {
  completed: boolean;
  marker_path?: string;
  completed_at?: string;
  skipped_agents?: boolean;
};

async function onboardingGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API}${path}`, { headers: { Accept: "application/json" } });
  const body = (await res.json().catch(() => ({}))) as T | { error?: string };
  if (!res.ok) {
    throw new Error((body as { error?: string }).error ?? res.statusText);
  }
  return body as T;
}

async function onboardingPost<T>(path: string, payload?: unknown): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      ...(payload !== undefined ? { "Content-Type": "application/json" } : {}),
    },
    body: payload !== undefined ? JSON.stringify(payload) : undefined,
  });
  const body = (await res.json().catch(() => ({}))) as T | { error?: string };
  if (!res.ok) {
    throw new Error((body as { error?: string }).error ?? res.statusText);
  }
  return body as T;
}

/** Whether first-run onboarding finished (onboarding.complete marker file on disk). */
export async function fetchOnboardingCompleted(): Promise<boolean> {
  try {
    const status = await onboardingGet<OnboardingStatus>("/status");
    return status.completed;
  } catch {
    if (isOnboardingDemo()) {
      try {
        return localStorage.getItem(COMPLETE_KEY) === "1";
      } catch {
        return false;
      }
    }
    return false;
  }
}

/** Write onboarding.complete (fallback to localStorage in UI-only demo). */
export async function markOnboardingComplete(options?: {
  skippedAgents?: boolean;
}): Promise<void> {
  clearOnboardingState();
  try {
    await onboardingPost("/complete", {
      skipped_agents: options?.skippedAgents ?? false,
    });
    return;
  } catch (err) {
    if (isOnboardingDemo()) {
      localStorage.setItem(COMPLETE_KEY, "1");
      return;
    }
    throw err;
  }
}

/** Remove onboarding.complete (and local demo state). */
export async function resetOnboardingComplete(): Promise<void> {
  clearOnboardingState();
  try {
    localStorage.removeItem(COMPLETE_KEY);
  } catch {
    // ignore
  }
  try {
    await onboardingPost("/reset");
  } catch {
    if (!isOnboardingDemo()) {
      throw new Error("Could not reset onboarding — is rmbd running?");
    }
  }
}
