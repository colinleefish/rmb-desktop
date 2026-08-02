import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { OverviewRoute } from "./pages/index";
import { MemoriesPage } from "./pages/MemoriesPage";
import { SessionDetailPage } from "./pages/SessionDetailPage";
import { SessionsPage } from "./pages/SessionsPage";
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
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
