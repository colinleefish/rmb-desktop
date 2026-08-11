/** Dev-only flags for onboarding wizard mock modes. */
import { clearOnboardingState } from "./onboardingState";
import { resetOnboardingComplete } from "./onboardingComplete";

export function isOnboardingDemo(): boolean {
  if (import.meta.env.VITE_MOCK_ONBOARDING === "true") return true;
  if (!import.meta.env.DEV) return false;
  try {
    return localStorage.getItem("rmb.mockOnboarding") === "1";
  } catch {
    return false;
  }
}

export function resetOnboardingDemo(): void {
  void resetOnboardingComplete().then(() => {
    clearOnboardingState();
  });
}
