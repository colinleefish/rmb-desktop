import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";

export function Layout() {
  return (
    <div className="flex h-screen overflow-hidden bg-rmb-light">
      <Sidebar />
      <main className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        <div className="w-full px-6 py-8 lg:px-8">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
