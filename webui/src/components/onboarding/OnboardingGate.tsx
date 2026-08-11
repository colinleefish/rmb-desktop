import { useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { isOnboardingComplete, isOnboardingDemo } from "../../lib/onboardingMock";

/** Redirects to onboarding wizard in demo mode until the user finishes or skips. */
export function OnboardingGate({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();

  useEffect(() => {
    if (!isOnboardingDemo()) return;
    if (isOnboardingComplete()) return;
    if (location.pathname.startsWith("/onboarding")) return;
    navigate("/onboarding", { replace: true });
  }, [location.pathname, navigate]);

  return children;
}
