import type { ReactNode } from "react";
import { useI18n } from "../../i18n";
import { isOnboardingDemo, resetOnboardingDemo } from "../../lib/onboardingMock";

export function OnboardingLayout({
  children,
  wide = false,
}: {
  children: ReactNode;
  wide?: boolean;
}) {
  const { t } = useI18n();

  return (
    <div className="h-screen overflow-y-auto bg-rmb-light">
      <div
        className={[
          "mx-auto px-4 py-10 sm:px-6",
          wide ? "max-w-5xl" : "max-w-4xl",
        ].join(" ")}
      >
        <header className="mb-8 text-center">
          <img src="/ui/logo.svg" alt="" className="mx-auto h-10 w-10" />
          <h1 className="mt-4 text-2xl font-semibold text-rmb-dark">{t.onboarding.title}</h1>
          <p className="mt-2 text-sm text-rmb-gray">{t.onboarding.subtitle}</p>
        </header>

        {isOnboardingDemo() && (
          <div className="mb-6 space-y-2">
            <p className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              {t.onboarding.demoBanner}
            </p>
            <button
              type="button"
              onClick={() => {
                resetOnboardingDemo();
                window.location.href = "/ui/onboarding";
              }}
              className="text-xs text-amber-800 underline hover:text-amber-950"
            >
              {t.onboarding.resetDemo}
            </button>
          </div>
        )}

        <div className="rounded-xl border border-rmb-gray/35 bg-white p-6 shadow-sm sm:p-8">
          {children}
        </div>
      </div>
    </div>
  );
}
