import type { ConfigTestResult } from "./onboardingApi";
import {
  DEFAULT_EMBED_DIMENSION,
  isEmbedDimension,
  normalizeEmbedDimension,
} from "./embedDimensions";

export type OnboardingStep = 1 | 2 | 3;

export type OnboardingModelsDraft = {
  llmBase: string;
  llmModel: string;
  llmKey: string;
  embedBase: string;
  embedModel: string;
  embedKey: string;
  embedDims: number;
};

export type OnboardingState = {
  step: OnboardingStep;
  maxStep: OnboardingStep;
  modelsDraft?: OnboardingModelsDraft;
  modelsTestResult?: ConfigTestResult;
};

const STATE_KEY = "rmb.onboardingState";

const DEFAULT_LLM_MODEL = "";
const DEFAULT_EMBED_MODEL = "";

export function defaultModelsDraft(): OnboardingModelsDraft {
  return {
    llmBase: "",
    llmModel: DEFAULT_LLM_MODEL,
    llmKey: "",
    embedBase: "",
    embedModel: DEFAULT_EMBED_MODEL,
    embedKey: "",
    embedDims: DEFAULT_EMBED_DIMENSION,
  };
}

function defaultState(): OnboardingState {
  return { step: 1, maxStep: 1 };
}

function clampStep(value: number): OnboardingStep {
  if (value <= 1) return 1;
  if (value >= 3) return 3;
  return value as OnboardingStep;
}

function normalizeDraft(raw?: Partial<OnboardingModelsDraft>): OnboardingModelsDraft {
  const base = defaultModelsDraft();
  if (!raw) return base;
  return {
    llmBase: raw.llmBase ?? base.llmBase,
    llmModel: raw.llmModel ?? base.llmModel,
    llmKey: raw.llmKey ?? base.llmKey,
    embedBase: raw.embedBase ?? base.embedBase,
    embedModel: raw.embedModel ?? base.embedModel,
    embedKey: raw.embedKey ?? base.embedKey,
    embedDims: isEmbedDimension(raw.embedDims ?? 0)
      ? raw.embedDims!
      : normalizeEmbedDimension(raw.embedDims ?? base.embedDims),
  };
}

function normalizeState(raw: Partial<OnboardingState>): OnboardingState {
  const step = clampStep(raw.step ?? 1);
  const maxStep = clampStep(Math.max(raw.maxStep ?? step, step));
  return {
    step,
    maxStep,
    modelsDraft: raw.modelsDraft ? normalizeDraft(raw.modelsDraft) : undefined,
    modelsTestResult: raw.modelsTestResult,
  };
}

export function readOnboardingState(): OnboardingState {
  try {
    const raw = localStorage.getItem(STATE_KEY);
    if (!raw) return defaultState();
    return normalizeState(JSON.parse(raw) as Partial<OnboardingState>);
  } catch {
    return defaultState();
  }
}

export function writeOnboardingState(patch: Partial<OnboardingState>) {
  const current = readOnboardingState();
  const merged = normalizeState({ ...current, ...patch });
  localStorage.setItem(STATE_KEY, JSON.stringify(merged));
  return merged;
}

export function saveModelsDraft(draft: OnboardingModelsDraft) {
  writeOnboardingState({ modelsDraft: normalizeDraft(draft) });
}

export function saveModelsTestResult(result: ConfigTestResult | undefined) {
  writeOnboardingState({ modelsTestResult: result });
}

export function completeOnboardingStep(step: OnboardingStep): OnboardingState {
  const current = readOnboardingState();
  const nextStep = clampStep(step + 1);
  return writeOnboardingState({
    step: nextStep,
    maxStep: clampStep(Math.max(current.maxStep, nextStep)),
  });
}

export function goToOnboardingStep(step: OnboardingStep): OnboardingState | null {
  const current = readOnboardingState();
  if (step > current.maxStep) return null;
  return writeOnboardingState({ step });
}

export function clearOnboardingState() {
  localStorage.removeItem(STATE_KEY);
  localStorage.removeItem("rmb.onboardingModelsSaved");
}
