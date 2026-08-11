/** Dev-only: onboarding wizard uses mock config test + local completion state. */
import { clearOnboardingState } from "./onboardingState";
export function isOnboardingDemo(): boolean {
  if (import.meta.env.VITE_MOCK_ONBOARDING === "true") return true;
  if (!import.meta.env.DEV) return false;
  try {
    return localStorage.getItem("rmb.mockOnboarding") === "1";
  } catch {
    return false;
  }
}

const COMPLETE_KEY = "rmb.onboardingComplete";

export function isOnboardingComplete(): boolean {
  try {
    return localStorage.getItem(COMPLETE_KEY) === "1";
  } catch {
    return false;
  }
}

export function markOnboardingComplete(): void {
  localStorage.setItem(COMPLETE_KEY, "1");
  clearOnboardingState();
}

export function resetOnboardingDemo(): void {
  localStorage.removeItem(COMPLETE_KEY);
  clearOnboardingState();
}
