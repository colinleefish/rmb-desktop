import { useEffect, useRef, useState } from "react";
import { AlertCircle, CheckCircle2, Info, Loader2, Lock, XCircle } from "lucide-react";
import { useI18n } from "../../i18n";
import { EmbedDimensionsSelect } from "../../components/EmbedDimensionsSelect";
import {
  normalizeEmbedDimension,
  type EmbedDimension,
} from "../../lib/embedDimensions";
import {
  defaultModelsDraft,
  readOnboardingState,
  saveModelsDraft,
  saveModelsTestResult,
} from "../../lib/onboardingState";
import {
  saveOnboardingModels,
  testEmbedConfig,
  testLLMConfig,
  type ConfigTestResult,
  type ConfigTestSide,
} from "../../lib/onboardingApi";

type TestSideState =
  | { status: "idle" }
  | { status: "testing" }
  | { status: "done"; result: ConfigTestSide };

function sideFromStored(result?: ConfigTestSide): TestSideState {
  return result ? { status: "done", result } : { status: "idle" };
}

export function OnboardingModelsStep({
  onComplete,
  onDraftPersisted,
}: {
  onComplete: () => void;
  onDraftPersisted?: () => void;
}) {
  const { t } = useI18n();
  const stored = readOnboardingState();
  const initialDraft = stored.modelsDraft ?? defaultModelsDraft();

  const [llmBase, setLlmBase] = useState(initialDraft.llmBase);
  const [llmModel, setLlmModel] = useState(initialDraft.llmModel);
  const [llmKey, setLlmKey] = useState(initialDraft.llmKey);
  const [embedBase, setEmbedBase] = useState(initialDraft.embedBase);
  const [embedModel, setEmbedModel] = useState(initialDraft.embedModel);
  const [embedDims, setEmbedDims] = useState<EmbedDimension>(
    normalizeEmbedDimension(initialDraft.embedDims),
  );
  const [embedKey, setEmbedKey] = useState(initialDraft.embedKey);

  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [llmTest, setLlmTest] = useState<TestSideState>(
    sideFromStored(stored.modelsTestResult?.llm),
  );
  const [embedTest, setEmbedTest] = useState<TestSideState>(
    sideFromStored(stored.modelsTestResult?.embed),
  );
  const [error, setError] = useState<string | null>(null);
  const skipTestClearRef = useRef(true);

  const payload = {
    llm: { api_base: llmBase, api_key: llmKey, model: llmModel },
    embed: {
      api_base: embedBase,
      api_key: embedKey,
      model: embedModel,
      dimensions: embedDims,
    },
  };

  const allTestsPassed =
    llmTest.status === "done" &&
    embedTest.status === "done" &&
    llmTest.result.ok === true &&
    embedTest.result.ok === true;

  const showTestPanel =
    llmTest.status !== "idle" || embedTest.status !== "idle";

  const testFinished =
    llmTest.status === "done" && embedTest.status === "done";

  useEffect(() => {
    saveModelsDraft({
      llmBase,
      llmModel,
      llmKey,
      embedBase,
      embedModel,
      embedKey,
      embedDims,
    });
    onDraftPersisted?.();

    if (skipTestClearRef.current) {
      skipTestClearRef.current = false;
      return;
    }
    setLlmTest({ status: "idle" });
    setEmbedTest({ status: "idle" });
    saveModelsTestResult(undefined);
  }, [
    llmBase,
    llmModel,
    llmKey,
    embedBase,
    embedModel,
    embedKey,
    embedDims,
    onDraftPersisted,
  ]);

  async function handleTest() {
    setTesting(true);
    setError(null);
    setLlmTest({ status: "testing" });
    setEmbedTest({ status: "testing" });
    saveModelsTestResult(undefined);

    let llmResult: ConfigTestSide = { ok: false, error: "Test did not run" };
    let embedResult: ConfigTestSide = { ok: false, error: "Test did not run" };

    const llmPromise = testLLMConfig(payload.llm)
      .then((result) => {
        llmResult = result;
        setLlmTest({ status: "done", result });
        return result;
      })
      .catch((err: unknown) => {
        const result: ConfigTestSide = {
          ok: false,
          error: err instanceof Error ? err.message : String(err),
        };
        llmResult = result;
        setLlmTest({ status: "done", result });
        return result;
      });

    const embedPromise = testEmbedConfig(payload.embed)
      .then((result) => {
        embedResult = result;
        setEmbedTest({ status: "done", result });
        return result;
      })
      .catch((err: unknown) => {
        const result: ConfigTestSide = {
          ok: false,
          error: err instanceof Error ? err.message : String(err),
        };
        embedResult = result;
        setEmbedTest({ status: "done", result });
        return result;
      });

    try {
      await Promise.all([llmPromise, embedPromise]);
      const combined: ConfigTestResult = { llm: llmResult, embed: embedResult };
      saveModelsTestResult(combined);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setTesting(false);
    }
  }

  async function handleContinue() {
    if (!allTestsPassed) return;
    setSaving(true);
    setError(null);
    try {
      await saveOnboardingModels(payload);
      onComplete();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-rmb-dark">{t.onboarding.models.title}</h2>
        <p className="mt-1 text-sm text-rmb-gray">{t.onboarding.models.intro}</p>
      </div>

      <section className="space-y-4">
        <h3 className="text-sm font-semibold text-rmb-dark">{t.settings.models.llm}</h3>
        <Field
          label={t.settings.llm.apiBase}
          value={llmBase}
          onChange={setLlmBase}
          placeholder={t.settings.llm.apiBasePlaceholder}
        />
        <Field
          label={t.settings.llm.model}
          value={llmModel}
          onChange={setLlmModel}
          placeholder={t.onboarding.models.llmModelPlaceholder}
        />
        <PasswordField
          label={t.settings.llm.apiKey}
          value={llmKey}
          onChange={setLlmKey}
          placeholder={t.onboarding.models.llmKeyPlaceholder}
        />
      </section>

      <section className="space-y-4 border-t border-rmb-gray/15 pt-6">
        <h3 className="text-sm font-semibold text-rmb-dark">{t.settings.models.embed}</h3>

        <EmbedSetupNotice />

        <Field
          label={t.settings.embed.apiBase}
          value={embedBase}
          onChange={setEmbedBase}
          placeholder={t.settings.embed.apiBasePlaceholder}
        />
        <Field
          label={t.settings.embed.model}
          value={embedModel}
          onChange={setEmbedModel}
          placeholder={t.onboarding.models.embedModelPlaceholder}
        />
        <div>
          <label
            id="onboarding-embed-dimensions-label"
            className="block text-sm font-medium text-rmb-gray"
          >
            {t.settings.embed.dimensions}
          </label>
          <div className="mt-1">
            <EmbedDimensionsSelect
              id="onboarding-embed-dimensions"
              labelId="onboarding-embed-dimensions-label"
              value={embedDims}
              onChange={setEmbedDims}
            />
          </div>
          <p className="mt-1.5 flex items-start gap-2 text-xs text-rmb-gray">
            <Lock className="mt-0.5 size-3.5 shrink-0 text-rmb-gray/70" aria-hidden />
            <span>{t.onboarding.models.embedNoticeDimensions}</span>
          </p>
        </div>
        <PasswordField
          label={t.settings.embed.apiKey}
          value={embedKey}
          onChange={setEmbedKey}
          placeholder={t.onboarding.models.embedKeyPlaceholder}
        />
      </section>

      {showTestPanel && (
        <div className="space-y-2 rounded-lg border border-rmb-gray/15 bg-rmb-light/40 p-4">
          <p className="text-sm font-medium text-rmb-dark">{t.onboarding.models.testResults}</p>
          {testing && (
            <p className="text-xs text-rmb-gray">{t.onboarding.models.testingHint}</p>
          )}
          <TestResultRow label={t.settings.models.llm} state={llmTest} showModelsList />
          <TestResultRow label={t.settings.models.embed} state={embedTest} />
        </div>
      )}

      {error && <p className="text-sm text-red-600">{error}</p>}

      <div className="flex flex-wrap items-center gap-3 border-t border-rmb-gray/15 pt-6">
        <button
          type="button"
          onClick={() => void handleTest()}
          disabled={testing}
          className="rounded-md border border-rmb-gray/25 bg-white px-4 py-2 text-sm font-medium text-rmb-dark hover:bg-rmb-light disabled:opacity-50"
        >
          {testing ? (
            <span className="inline-flex items-center gap-2">
              <Loader2 className="size-4 animate-spin" />
              {t.onboarding.models.testing}
            </span>
          ) : (
            t.onboarding.models.testConnection
          )}
        </button>
        <button
          type="button"
          onClick={() => void handleContinue()}
          disabled={!allTestsPassed || saving}
          className="rounded-md bg-rmb-accent px-4 py-2 text-sm font-medium text-white hover:bg-rmb-accent/90 disabled:opacity-50"
        >
          {saving ? t.onboarding.models.saving : t.onboarding.models.continue}
        </button>
        {!allTestsPassed && testFinished && (
          <p className="text-xs text-rmb-gray">{t.onboarding.models.fixErrorsHint}</p>
        )}
      </div>
    </div>
  );
}

function EmbedSetupNotice() {
  const { t } = useI18n();

  return (
    <div className="rounded-lg border border-amber-200/90 bg-amber-50/90 px-4 py-3">
      <div className="flex gap-3">
        <Info className="mt-0.5 size-4 shrink-0 text-amber-800" aria-hidden />
        <div className="min-w-0 space-y-2 text-sm text-amber-950">
          <p className="font-medium">{t.onboarding.models.embedNoticeTitle}</p>
          <ul className="space-y-1.5 text-xs leading-relaxed text-amber-900/90">
            <li className="flex items-start gap-2">
              <Lock className="mt-0.5 size-3.5 shrink-0" aria-hidden />
              <span>{t.onboarding.models.embedNoticeDimensions}</span>
            </li>
            <li className="flex items-start gap-2">
              <AlertCircle className="mt-0.5 size-3.5 shrink-0" aria-hidden />
              <span>{t.onboarding.models.embedNoticeModel}</span>
            </li>
          </ul>
        </div>
      </div>
    </div>
  );
}

function TestResultRow({
  label,
  state,
  showModelsList = false,
}: {
  label: string;
  state: TestSideState;
  showModelsList?: boolean;
}) {
  const { t } = useI18n();

  if (state.status === "idle") {
    return null;
  }

  if (state.status === "testing") {
    return (
      <div className="flex items-start gap-2 text-sm">
        <Loader2 className="mt-0.5 size-4 shrink-0 animate-spin text-rmb-gray" aria-hidden />
        <div>
          <span className="font-medium text-rmb-dark">{label}</span>
          <span className="text-rmb-gray"> — {t.onboarding.models.testing}</span>
        </div>
      </div>
    );
  }

  const result = state.result;
  const requested = result.requested_model?.trim();
  const models = result.models ?? [];
  const showList = showModelsList && models.length > 0;

  return (
    <div className="space-y-1.5">
      <div className="flex items-start gap-2 text-sm">
        {result.ok ? (
          <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600" />
        ) : (
          <XCircle className="mt-0.5 size-4 shrink-0 text-red-600" />
        )}
        <div className="min-w-0">
          <span className="font-medium text-rmb-dark">{label}</span>
          {result.ok ? (
            <span className="text-rmb-gray">
              {" "}
              — {t.onboarding.models.testOk}
              {result.latency_ms != null && ` (${result.latency_ms} ms)`}
              {requested && result.model_found && (
                <span className="text-emerald-700">
                  {" "}
                  — {t.onboarding.models.testModelInList.replace("{model}", requested)}
                </span>
              )}
            </span>
          ) : (
            <span className="text-red-600"> — {result.error ?? t.onboarding.models.testFailed}</span>
          )}
        </div>
      </div>

      {showList && (
        <div className="ml-6 space-y-1">
          <p className="text-xs text-rmb-gray">
            {t.onboarding.models.testModelsPreview.replace(
              "{count}",
              String(result.models_count ?? models.length),
            )}
          </p>
          <ul
            className="max-h-28 overflow-y-auto rounded border border-rmb-gray/15 bg-white/80 px-2 py-1.5 text-xs text-rmb-gray"
            aria-label={t.onboarding.models.testModelsListLabel}
          >
            {models.map((id) => {
              const isMatch = requested && id === requested;
              return (
                <li
                  key={id}
                  className={
                    isMatch
                      ? "font-medium text-emerald-700"
                      : requested && result.model_found === false
                        ? "text-rmb-gray/80"
                        : undefined
                  }
                >
                  {id}
                  {isMatch ? ` ${t.onboarding.models.testModelMatchMark}` : ""}
                </li>
              );
            })}
          </ul>
          {requested && result.model_found === false && (
            <p className="text-xs text-red-600">
              {t.onboarding.models.testModelNotInList.replace("{model}", requested)}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

const fieldClassName =
  "mt-1 w-full rounded-md border border-rmb-gray/20 px-3 py-2 text-sm text-rmb-dark";

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-rmb-gray">{label}</label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`${fieldClassName} placeholder:text-rmb-gray/45`}
      />
    </div>
  );
}

function PasswordField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-rmb-gray">{label}</label>
      <input
        type="password"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`${fieldClassName} placeholder:text-rmb-gray/45`}
      />
    </div>
  );
}
