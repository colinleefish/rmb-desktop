import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { IntegrationSettingsPanel } from "../components/settings/IntegrationSettingsPanel";
import { useI18n } from "../i18n";
import { integrationPath, parseIntegrationPath } from "../lib/integrationRoutes";
import { isSetupMocked } from "../lib/setupApi";

export function IntegrationsPage() {
  const { t } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const { agentId } = parseIntegrationPath(location.pathname);

  if (location.pathname === "/integrations" || location.pathname === "/integrations/") {
    return <Navigate to="/integrations/cursor" replace />;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">{t.agents.hubTitle}</h1>
        <p className="mt-1 text-rmb-gray">{t.agents.hubSubtitle}</p>
      </div>

      <div className="overflow-hidden rounded-xl border border-rmb-gray/35 bg-white shadow-sm">
        <div className="p-6">
          {isSetupMocked() && (
            <p className="mb-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              {t.agents.previewBanner}
            </p>
          )}
          <IntegrationSettingsPanel
            agentId={agentId}
            onAgentChange={(id) => navigate(integrationPath(id), { replace: true })}
          />
        </div>
      </div>
    </div>
  );
}
