import { Outlet } from "react-router-dom";
import { OverviewCountsProvider } from "../lib/overviewCounts";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";

export function Layout() {
  return (
    <OverviewCountsProvider>
    <div className="flex h-screen overflow-hidden bg-white text-rmb-dark">
      <Sidebar />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-white">
        <Topbar />
        <main className="min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto w-full max-w-7xl px-8 py-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
    </OverviewCountsProvider>
  );
}
