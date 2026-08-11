import { useEffect, useRef, useState } from "react";
import { CheckCircle2, Loader2, XCircle } from "lucide-react";
import { useI18n } from "../i18n";
import {
  testEmbedConfig,
  testLLMConfig,
  type ConfigTestSide,
} from "../lib/onboardingApi";

export type TestSideState =
  | { status: "idle" }
  | { status: "testing" }
  | { status: "done"; result: ConfigTestSide };

type ModelsConnectionTestPanelProps = {
  llm: { api_base: string; api_key: string; model: string };
  embed: {
    api_base: string;
    api_key: string;
    model: string;
    dimensions: number;
  };
};

export function ModelsConnectionTestPanel({
  llm,
  embed,
}: ModelsConnectionTestPanelProps) {
  const { t } = useI18n();
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [llmTest, setLlmTest] = useState<TestSideState>({ status: "idle" });
  const [embedTest, setEmbedTest] = useState<TestSideState>({ status: "idle" });
  const skipClearRef = useRef(true);

  const showResults = llmTest.status !== "idle" || embedTest.status !== "idle";

  useEffect(() => {
    if (skipClearRef.current) {
      skipClearRef.current = false;
      return;
    }
    setLlmTest({ status: "idle" });
    setEmbedTest({ status: "idle" });
    setError(null);
  }, [
    llm.api_base,
    llm.api_key,
    llm.model,
    embed.api_base,
    embed.api_key,
    embed.model,
    embed.dimensions,
  ]);

  async function handleTest() {
    setTesting(true);
    setError(null);
    setLlmTest({ status: "testing" });
    setEmbedTest({ status: "testing" });

    const runSide = async (
      run: () => Promise<ConfigTestSide>,
      setSide: (state: TestSideState) => void,
    ) => {
      try {
        const result = await run();
        setSide({ status: "done", result });
        return result;
      } catch (err) {
        const result: ConfigTestSide = {
          ok: false,
          error: err instanceof Error ? err.message : String(err),
        };
        setSide({ status: "done", result });
        return result;
      }
    };

    try {
      await Promise.all([
        runSide(() => testLLMConfig(llm), setLlmTest),
        runSide(() => testEmbedConfig(embed), setEmbedTest),
      ]);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setTesting(false);
    }
  }

  return (
    <div className="space-y-4 border-t border-rmb-gray/15 pt-6">
      {showResults && (
        <div className="space-y-2 rounded-lg border border-rmb-gray/15 bg-rmb-light/40 p-4">
          <p className="text-sm font-medium text-rmb-dark">
            {t.settings.models.testResults}
          </p>
          <TestResultRow label={t.settings.models.llm} state={llmTest} />
          <TestResultRow label={t.settings.models.embed} state={embedTest} />
        </div>
      )}

      {error && <p className="text-sm text-red-600">{error}</p>}

      <button
        type="button"
        onClick={() => void handleTest()}
        disabled={testing}
        className="rounded-md border border-rmb-gray/25 bg-white px-4 py-2 text-sm font-medium text-rmb-dark hover:bg-rmb-light disabled:opacity-50"
      >
        {testing ? (
          <span className="inline-flex items-center gap-2">
            <Loader2 className="size-4 animate-spin" aria-hidden />
            {t.settings.models.testing}
          </span>
        ) : (
          t.settings.models.testConnection
        )}
      </button>
    </div>
  );
}

function TestResultRow({
  label,
  state,
}: {
  label: string;
  state: TestSideState;
}) {
  const { t } = useI18n();

  if (state.status === "idle") {
    return null;
  }

  if (state.status === "testing") {
    return (
      <div className="flex items-center gap-2 text-sm">
        <Loader2 className="size-4 shrink-0 animate-spin text-rmb-gray" aria-hidden />
        <span className="font-medium text-rmb-dark">{label}</span>
        <span className="text-rmb-gray">— {t.settings.models.testing}</span>
      </div>
    );
  }

  const result = state.result;

  return (
    <div className="flex items-center gap-2 text-sm">
      {result.ok ? (
        <CheckCircle2 className="size-4 shrink-0 text-emerald-600" aria-hidden />
      ) : (
        <XCircle className="size-4 shrink-0 text-red-600" aria-hidden />
      )}
      <span className="font-medium text-rmb-dark">{label}</span>
      {result.ok ? (
        <span className="text-emerald-700">— {t.settings.models.testOk}</span>
      ) : (
        <span className="text-red-600">
          — {result.error ?? t.settings.models.testFailed}
        </span>
      )}
    </div>
  );
}
