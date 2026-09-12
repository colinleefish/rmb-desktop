import { BrowserRouter, Navigate, Route, Routes, useLocation, useParams } from "react-router-dom";
import { OnboardingGate } from "./components/onboarding/OnboardingGate";
import { Layout } from "./components/Layout";
import { OverviewRoute } from "./pages/index";
import { MemoriesPage } from "./pages/MemoriesPage";
import { SessionDetailPage } from "./pages/SessionDetailPage";
import { SessionsPage } from "./pages/SessionsPage";
import { SkillDetailPage } from "./pages/SkillDetailPage";
import { AgentsPage } from "./pages/AgentsPage";
import { SettingsPage } from "./pages/SettingsPage";
import { OnboardingPage } from "./pages/onboarding/OnboardingPage";
import { isIntegrationAgentId } from "./lib/agentRegistry";
import { integrationPath } from "./lib/integrationRoutes";

function RedirectSkillSlug() {
  const { slug } = useParams();
  return <Navigate to={`/agents/skills/${encodeURIComponent(slug ?? "")}`} replace />;
}

function RedirectAgentId() {
  const { id } = useParams();
  return <Navigate to={`/agents/${encodeURIComponent(id ?? "")}`} replace />;
}

function RedirectSettingsIntegrations() {
  const { pathname } = useLocation();
  const raw = pathname.replace(/^\/settings\/integrations\/?/, "");
  const agentId = raw && isIntegrationAgentId(raw) ? raw : "cursor";
  return <Navigate to={integrationPath(agentId)} replace />;
}

export default function App() {
  return (
    <BrowserRouter basename={import.meta.env.BASE_URL.replace(/\/$/, "")}>
      <Routes>
        <Route path="onboarding" element={<OnboardingPage />} />
        <Route
          element={
            <OnboardingGate>
              <Layout />
            </OnboardingGate>
          }
        >
          <Route index element={<OverviewRoute />} />
          <Route path="sessions" element={<SessionsPage />} />
          <Route path="sessions/:sessionKey" element={<SessionDetailPage />} />
          <Route path="memories" element={<MemoriesPage />} />
          <Route path="memories/:category" element={<MemoriesPage />} />
          <Route path="agents" element={<AgentsPage />} />
          <Route path="agents/skills" element={<AgentsPage />} />
          <Route path="agents/skills/:slug" element={<SkillDetailPage />} />
          <Route path="agents/:agentId" element={<AgentsPage />} />
          <Route path="skills" element={<Navigate to="/agents/skills" replace />} />
          <Route path="skills/:slug" element={<RedirectSkillSlug />} />
          <Route path="pipeline" element={<Navigate to="/" replace />} />
          <Route path="integrations" element={<Navigate to="/agents" replace />} />
          <Route path="integrations/:id" element={<RedirectAgentId />} />
          <Route path="settings/integrations/*" element={<RedirectSettingsIntegrations />} />
          <Route path="settings/*" element={<SettingsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}


