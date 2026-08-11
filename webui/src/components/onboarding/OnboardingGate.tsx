import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { fetchOnboardingCompleted } from "../../lib/onboardingComplete";

/** Redirect to onboarding until onboarding.complete exists on disk. */
export function OnboardingGate({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      const completed = await fetchOnboardingCompleted();
      if (cancelled) return;
      setReady(true);
      if (completed) return;
      if (location.pathname.startsWith("/onboarding")) return;
      navigate("/onboarding", { replace: true });
    })();

    return () => {
      cancelled = true;
    };
  }, [location.pathname, navigate]);

  if (!ready) return null;

  return children;
}
