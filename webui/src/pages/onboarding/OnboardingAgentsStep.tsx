import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { IntegrationSettingsPanel } from "../../components/settings/IntegrationSettingsPanel";
import { useI18n } from "../../i18n";
import type { IntegrationAgentId } from "../../integrations/types";
import { isAgentConfigured } from "../../lib/agentStatus";
import { fetchSetupStatus } from "../../lib/setupApi";
import { markOnboardingComplete } from "../../lib/onboardingComplete";

export function OnboardingAgentsStep() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [agentId, setAgentId] = useState<IntegrationAgentId>("cursor");
  const [configuredCount, setConfiguredCount] = useState(0);

  const refreshConfiguredCount = useCallback(async () => {
    const agents = await fetchSetupStatus();
    setConfiguredCount(agents.filter((a) => a.detected && isAgentConfigured(a)).length);
  }, []);

  useEffect(() => {
    void refreshConfiguredCount();
  }, [refreshConfiguredCount]);

  async function handleFinish() {
    await markOnboardingComplete({ skippedAgents: false });
    navigate("/", { replace: true });
  }

  const canFinish = configuredCount > 0;

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

      {configuredCount === 0 && (
        <p className="text-sm text-amber-800">{t.onboarding.agents.needOneAgent}</p>
      )}

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
            void markOnboardingComplete({ skippedAgents: true }).then(() => {
              navigate("/", { replace: true });
            });
          }}
          className="text-sm text-rmb-gray hover:text-rmb-dark"
        >
          {t.onboarding.agents.skipForNow}
        </button>
      </div>
    </div>
  );
}
