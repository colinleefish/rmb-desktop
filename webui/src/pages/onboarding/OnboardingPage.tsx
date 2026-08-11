import { useState } from "react";
import { OnboardingProgress } from "../../components/onboarding/OnboardingProgress";
import { OnboardingLayout } from "./OnboardingLayout";
import { OnboardingLanguageStep } from "./OnboardingLanguageStep";
import { OnboardingModelsStep } from "./OnboardingModelsStep";
import { OnboardingAgentsStep } from "./OnboardingAgentsStep";
import {
  completeOnboardingStep,
  goToOnboardingStep,
  readOnboardingState,
  type OnboardingStep,
} from "../../lib/onboardingState";

export function OnboardingPage() {
  const initial = readOnboardingState();
  const [step, setStep] = useState<OnboardingStep>(initial.step);
  const [maxStep, setMaxStep] = useState<OnboardingStep>(initial.maxStep);

  function syncFromStorage() {
    const state = readOnboardingState();
    setStep(state.step);
    setMaxStep(state.maxStep);
  }

  function handleStepClick(target: OnboardingStep) {
    const next = goToOnboardingStep(target);
    if (!next) return;
    setStep(next.step);
    setMaxStep(next.maxStep);
  }

  function handleLanguageComplete() {
    const next = completeOnboardingStep(1);
    setStep(next.step);
    setMaxStep(next.maxStep);
  }

  function handleModelsComplete() {
    const next = completeOnboardingStep(2);
    setStep(next.step);
    setMaxStep(next.maxStep);
  }

  return (
    <OnboardingLayout wide={step === 3}>
      <OnboardingProgress
        step={step}
        maxStep={maxStep}
        onStepClick={handleStepClick}
      />
      {step === 1 && <OnboardingLanguageStep onComplete={handleLanguageComplete} />}
      {step === 2 && (
        <OnboardingModelsStep
          onComplete={handleModelsComplete}
          onDraftPersisted={syncFromStorage}
        />
      )}
      {step === 3 && <OnboardingAgentsStep />}
    </OnboardingLayout>
  );
}
