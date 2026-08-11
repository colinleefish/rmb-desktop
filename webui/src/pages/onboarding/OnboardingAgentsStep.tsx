import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { CheckCircle2, ExternalLink, Loader2 } from "lucide-react";
import { IntegrationSettingsPanel } from "../../components/settings/IntegrationSettingsPanel";
import { useI18n } from "../../i18n";
import type { IntegrationAgentId } from "../../integrations/types";
import { isAgentConfigured } from "../../lib/agentStatus";
import { fetchSetupStatus } from "../../lib/setupApi";
import { isOnboardingDemo, markOnboardingComplete } from "../../lib/onboardingMock";
import { pageSessions } from "../../lib/api";

export function OnboardingAgentsStep() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [agentId, setAgentId] = useState<IntegrationAgentId>("cursor");
  const [configuredCount, setConfiguredCount] = useState(0);
  const [verifying, setVerifying] = useState(false);
  const [verified, setVerified] = useState(false);
  const [simulatedVerify, setSimulatedVerify] = useState(false);

  const refreshConfiguredCount = useCallback(async () => {
    const agents = await fetchSetupStatus();
    setConfiguredCount(agents.filter((a) => a.detected && isAgentConfigured(a)).length);
  }, []);

  useEffect(() => {
    void refreshConfiguredCount();
  }, [refreshConfiguredCount]);

  async function pollSessions() {
    setVerifying(true);
    try {
      const page = await pageSessions({ limit: 5, offset: 0 });
      if (page.total > 0) {
        setVerified(true);
        return;
      }
    } catch {
      // rmbd may be offline in pure UI demo
    } finally {
      setVerifying(false);
    }
  }

  function handleSimulateVerify() {
    setSimulatedVerify(true);
    setVerified(true);
  }

  function handleFinish() {
    markOnboardingComplete();
    navigate("/", { replace: true });
  }

  const canFinish = configuredCount > 0 && (verified || simulatedVerify);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-rmb-dark">{t.onboarding.agents.title}</h2>
        <p className="mt-1 text-sm text-rmb-gray">{t.onboarding.agents.intro}</p>
      </div>

      <IntegrationSettingsPanel
        agentId={agentId}
        onAgentChange={setAgentId}
        onAgentsChange={(agents) =>
          setConfiguredCount(
            agents.filter((a) => a.detected && isAgentConfigured(a)).length,
          )
        }
      />

      <section className="space-y-4 border-t border-rmb-gray/15 pt-6">
        <div>
          <h3 className="text-sm font-semibold text-rmb-dark">{t.agents.verify}</h3>
          <p className="mt-1 text-sm text-rmb-gray">{t.onboarding.agents.verifyIntro}</p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => void pollSessions()}
            disabled={verifying}
            className="rounded-md border border-rmb-gray/25 bg-white px-4 py-2 text-sm font-medium text-rmb-dark hover:bg-rmb-light disabled:opacity-50"
          >
            {verifying ? (
              <span className="inline-flex items-center gap-2">
                <Loader2 className="size-4 animate-spin" />
                {t.onboarding.agents.checking}
              </span>
            ) : (
              t.onboarding.agents.checkSessions
            )}
          </button>

          {isOnboardingDemo() && (
            <button
              type="button"
              onClick={handleSimulateVerify}
              className="rounded-md border border-dashed border-amber-300 bg-amber-50 px-4 py-2 text-sm font-medium text-amber-900 hover:bg-amber-100"
            >
              {t.onboarding.agents.simulateVerify}
            </button>
          )}

          <Link
            to={`/sessions?source=${agentId}`}
            className="inline-flex items-center gap-1.5 rounded-md border border-rmb-gray/20 bg-white px-3 py-2 text-sm font-medium text-rmb-dark hover:border-rmb-accent/40 hover:text-rmb-accent"
          >
            {t.agents.openSessions}
            <ExternalLink className="size-3.5" />
          </Link>
        </div>

        {(verified || simulatedVerify) && (
          <p className="flex items-center gap-2 text-sm text-emerald-700">
            <CheckCircle2 className="size-4" />
            {simulatedVerify && !verified
              ? t.onboarding.agents.verifySimulated
              : t.onboarding.agents.verifySuccess}
          </p>
        )}

        {configuredCount === 0 && (
          <p className="text-sm text-amber-800">{t.onboarding.agents.needOneAgent}</p>
        )}
      </section>

      <div className="flex flex-wrap items-center gap-3 border-t border-rmb-gray/15 pt-6">
        <button
          type="button"
          onClick={() => void handleFinish()}
          disabled={!canFinish}
          className="rounded-md bg-rmb-accent px-4 py-2 text-sm font-medium text-white hover:bg-rmb-accent/90 disabled:opacity-50"
        >
          {t.onboarding.agents.finish}
        </button>
        <button
          type="button"
          onClick={() => {
            markOnboardingComplete();
            navigate("/", { replace: true });
          }}
          className="text-sm text-rmb-gray hover:text-rmb-dark"
        >
          {t.onboarding.agents.skipForNow}
        </button>
      </div>
    </div>
  );
}
