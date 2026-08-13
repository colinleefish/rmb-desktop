import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { OnboardingGate } from "./components/onboarding/OnboardingGate";
import { Layout } from "./components/Layout";
import { OverviewRoute } from "./pages/index";
import { MemoriesPage } from "./pages/MemoriesPage";
import { SessionDetailPage } from "./pages/SessionDetailPage";
import { SessionsPage } from "./pages/SessionsPage";
import { SkillsPage } from "./pages/SkillsPage";
import { SkillDetailPage } from "./pages/SkillDetailPage";
import { IntegrationsPage } from "./pages/IntegrationsPage";
import { RedirectSettingsIntegrations } from "./pages/RedirectSettingsIntegrations";
import { SettingsPage } from "./pages/SettingsPage";
import { OnboardingPage } from "./pages/onboarding/OnboardingPage";

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
          <Route path="memories" element={<Navigate to="/memories/profile" replace />} />
          <Route path="memories/:category" element={<MemoriesPage />} />
          <Route path="skills" element={<SkillsPage />} />
          <Route path="skills/:slug" element={<SkillDetailPage />} />
          <Route path="pipeline" element={<Navigate to="/" replace />} />
          <Route path="integrations/*" element={<IntegrationsPage />} />
          <Route path="settings/integrations/*" element={<RedirectSettingsIntegrations />} />
          <Route path="settings/*" element={<SettingsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
