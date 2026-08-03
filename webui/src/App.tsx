import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { OverviewRoute } from "./pages/index";
import { MemoriesPage } from "./pages/MemoriesPage";
import { SessionDetailPage } from "./pages/SessionDetailPage";
import { SessionsPage } from "./pages/SessionsPage";
import { SkillsPage } from "./pages/SkillsPage";
import { SkillDetailPage } from "./pages/SkillDetailPage";
import { SettingsPage } from "./pages/SettingsPage";

export default function App() {
  return (
    <BrowserRouter basename="/ui">
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<OverviewRoute />} />
          <Route path="sessions" element={<SessionsPage />} />
          <Route path="sessions/:sessionKey" element={<SessionDetailPage />} />
          <Route path="memories" element={<Navigate to="/memories/profile" replace />} />
          <Route path="memories/:category" element={<MemoriesPage />} />
          <Route path="skills" element={<SkillsPage />} />
          <Route path="skills/:slug" element={<SkillDetailPage />} />
          <Route path="integrations/cursor" element={<Navigate to="/settings/integrations/cursor" replace />} />
          <Route path="integrations/claude-code" element={<Navigate to="/settings/integrations/claude-code" replace />} />
          <Route path="settings/*" element={<SettingsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
