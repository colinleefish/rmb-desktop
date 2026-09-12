import { useCallback, useState } from "react";

export const SIDEBAR_COLLAPSED_KEY = "rmb.sidebarCollapsed";

export function readSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "1";
  } catch {
    return false;
  }
}

export function writeSidebarCollapsed(collapsed: boolean): void {
  try {
    localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? "1" : "0");
  } catch {
    /* ignore quota / private mode */
  }
}

export function useSidebarCollapsed() {
  const [collapsed, setCollapsedState] = useState(readSidebarCollapsed);

  const setCollapsed = useCallback((next: boolean) => {
    writeSidebarCollapsed(next);
    setCollapsedState(next);
  }, []);

  const toggle = useCallback(() => {
    setCollapsed(!collapsed);
  }, [collapsed, setCollapsed]);

  return { collapsed, setCollapsed, toggle };
}
