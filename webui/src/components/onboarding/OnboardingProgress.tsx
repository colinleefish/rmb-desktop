import { useI18n } from "../../i18n";
import type { OnboardingStep } from "../../lib/onboardingState";

export function OnboardingProgress({
  step,
  maxStep,
  onStepClick,
}: {
  step: OnboardingStep;
  maxStep: OnboardingStep;
  onStepClick: (step: OnboardingStep) => void;
}) {
  const { t } = useI18n();
  const steps: { id: OnboardingStep; label: string }[] = [
    { id: 1, label: t.onboarding.steps.language },
    { id: 2, label: t.onboarding.steps.models },
    { id: 3, label: t.onboarding.steps.agents },
  ];

  return (
    <nav aria-label={t.onboarding.progressLabel} className="mb-8">
      <ol className="flex flex-wrap items-center justify-center gap-x-2 gap-y-2 sm:justify-between sm:gap-x-0">
        {steps.map((item, index) => {
          const active = step === item.id;
          const done = step > item.id;
          const reachable = item.id <= maxStep;
          return (
            <li key={item.id} className="flex items-center sm:flex-1 sm:last:flex-none">
              <button
                type="button"
                disabled={!reachable}
                onClick={() => onStepClick(item.id)}
                className={[
                  "flex items-center gap-2 rounded-md px-1 py-0.5 text-left transition",
                  reachable ? "cursor-pointer hover:bg-rmb-light/80" : "cursor-default",
                  active ? "bg-rmb-accent/5" : "",
                ].join(" ")}
              >
                <span
                  className={[
                    "flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold transition",
                    done
                      ? "bg-emerald-500 text-white"
                      : active
                        ? "bg-rmb-accent text-white"
                        : "border border-rmb-gray/25 bg-white text-rmb-gray",
                  ].join(" ")}
                >
                  {done ? "✓" : item.id}
                </span>
                <span
                  className={[
                    "text-sm font-medium",
                    active || done ? "text-rmb-dark" : "text-rmb-gray/60",
                    reachable && !active ? "hover:text-rmb-accent" : "",
                  ].join(" ")}
                >
                  {item.label}
                </span>
              </button>
              {index < steps.length - 1 && (
                <span
                  className="mx-3 hidden h-px flex-1 bg-rmb-gray/25 sm:mx-4 sm:block"
                  aria-hidden
                />
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
