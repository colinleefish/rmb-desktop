import { Navigate, useLocation } from "react-router-dom";
import { isIntegrationAgentId } from "../lib/agentRegistry";
import { integrationPath } from "../lib/integrationRoutes";

export function RedirectSettingsIntegrations() {
  const { pathname } = useLocation();
  const raw = pathname.replace(/^\/settings\/integrations\/?/, "");
  const agentId = raw && isIntegrationAgentId(raw) ? raw : "cursor";
  return <Navigate to={integrationPath(agentId)} replace />;
}
