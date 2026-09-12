import { NavLink, useLocation } from "react-router-dom";
import type { ComponentType } from "react";
import {
  Bot,
  ChevronLeft,
  ChevronRight,
  LayoutDashboard,
  MessagesSquare,
  Settings,
  Sparkles,
} from "lucide-react";
import { useEffect, useState } from "react";
import { probeDaemon } from "../lib/api";
import { isFullMock } from "../lib/mockMode";
import { useOverviewCounts } from "../lib/overviewCounts";
import { useSidebarCollapsed } from "../lib/sidebarCollapsed";
import type { OverviewCounts } from "../lib/types";
import { useI18n } from "../i18n";

type NavItem = {
  to: string;
  label: string;
  icon: ComponentType<{ className?: string; strokeWidth?: number }>;
  countKey?: keyof OverviewCounts;
  end?: boolean;
  activePrefix?: string;
  extraPrefixes?: string[];
};

function navLinkClass(isActive: boolean, collapsed: boolean) {
  return [
    "flex h-8 items-center rounded-md text-sm transition-colors",
    collapsed ? "justify-center px-0" : "gap-2.5 px-2.5",
    isActive
      ? "bg-rmb-fill font-medium text-rmb-dark [&_svg]:stroke-rmb-accent"
      : "text-rmb-muted hover:bg-rmb-fill hover:text-rmb-dark [&_svg]:stroke-rmb-faint",
  ].join(" ");
}

function pathActive(pathname: string, item: NavItem, navActive: boolean): boolean {
  if (item.end) return navActive;
  if (navActive) return true;
  if (item.activePrefix && pathname.startsWith(item.activePrefix)) return true;
  return Boolean(item.extraPrefixes?.some((p) => pathname.startsWith(p)));
}

function navTooltip(label: string, count: number | null, collapsed: boolean): string | undefined {
  if (!collapsed) return undefined;
  if (count != null) return `${label} (${count})`;
  return label;
}

function SidebarNavItem({
  item,
  counts,
  collapsed,
}: {
  item: NavItem;
  counts: OverviewCounts | null;
  collapsed: boolean;
}) {
  const Icon = item.icon;
  const location = useLocation();
  const count =
    item.countKey && counts != null ? counts[item.countKey] : null;
  const tooltip = navTooltip(item.label, count, collapsed);

  return (
    <NavLink
      to={item.to}
      end={item.end}
      title={tooltip}
      className={({ isActive }) =>
        navLinkClass(pathActive(location.pathname, item, isActive), collapsed)
      }
    >
      {({ isActive }) => {
        const highlighted = pathActive(location.pathname, item, isActive);
        return (
          <>
            <Icon className="size-4 shrink-0" strokeWidth={1.75} />
            {!collapsed ? (
              <>
                <span className="flex-1">{item.label}</span>
                {count != null ? (
                  <span
                    className={`text-[11px] ${highlighted ? "text-rmb-muted" : "text-rmb-faint"}`}
                  >
                    {count}
                  </span>
                ) : null}
              </>
            ) : null}
          </>
        );
      }}
    </NavLink>
  );
}

export function Sidebar() {
  const { t } = useI18n();
  const location = useLocation();
  const counts = useOverviewCounts();
  const { collapsed, toggle } = useSidebarCollapsed();
  const [daemonUp, setDaemonUp] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    const tick = () => {
      probeDaemon()
        .then((ok) => {
          if (!cancelled) setDaemonUp(ok);
        })
        .catch(() => {
          if (!cancelled) setDaemonUp(false);
        });
    };
    tick();
    const id = window.setInterval(tick, 5000);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  const items: NavItem[] = [
    { to: "/", label: t.nav.overview, icon: LayoutDashboard, end: true },
    {
      to: "/sessions",
      label: t.nav.sessions,
      icon: MessagesSquare,
      countKey: "sessions",
    },
    {
      to: "/memories",
      label: t.nav.memories,
      icon: Sparkles,
      countKey: "memories",
      activePrefix: "/memories",
    },
    {
      to: "/agents",
      label: t.nav.agents,
      icon: Bot,
      activePrefix: "/agents",
      extraPrefixes: ["/integrations", "/skills"],
    },
  ];

  const daemonLabel = !daemonUp
    ? t.nav.daemonDown
    : isFullMock()
      ? t.nav.daemonMock
      : t.nav.daemonLocal;

  const toggleLabel = collapsed ? t.nav.expandSidebar : t.nav.collapseSidebar;
  const ToggleIcon = collapsed ? ChevronRight : ChevronLeft;

  return (
    <div
      className={[
        "relative flex h-full shrink-0 flex-col transition-[width] duration-200 ease-in-out",
        collapsed ? "w-14" : "w-56",
      ].join(" ")}
    >
      <aside className="flex h-full flex-col overflow-hidden border-r border-rmb-line bg-white">
      <div
        className={[
          "flex h-14 shrink-0 items-center",
          collapsed ? "justify-center px-1" : "gap-2 px-3",
        ].join(" ")}
      >
        <img
          src={`${import.meta.env.BASE_URL}logo.svg`}
          alt=""
          className={[
            "shrink-0 rounded-md",
            collapsed ? "size-6" : "size-7",
          ].join(" ")}
          width={collapsed ? 24 : 28}
          height={collapsed ? 24 : 28}
        />
        {!collapsed ? (
          <div className="min-w-0 flex-1 leading-tight">
            <div className="truncate text-sm font-semibold text-rmb-dark">{t.appName}</div>
            {t.appSubtitle ? (
              <div className="truncate text-xs text-rmb-muted">{t.appSubtitle}</div>
            ) : null}
          </div>
        ) : null}
      </div>

      <nav className={["flex-1 overflow-y-auto overflow-x-hidden pt-2", collapsed ? "px-1.5" : "px-3"].join(" ")}>
        <ul className="space-y-0.5">
          {items.map((item) => (
            <li key={item.to}>
              <SidebarNavItem item={item} counts={counts} collapsed={collapsed} />
            </li>
          ))}
        </ul>
      </nav>

      <div className={["space-y-1 pb-3", collapsed ? "px-1.5" : "px-3"].join(" ")}>
        <NavLink
          to="/settings/general"
          title={collapsed ? t.nav.settings : undefined}
          className={({ isActive }) =>
            navLinkClass(
              isActive || location.pathname.startsWith("/settings"),
              collapsed,
            )
          }
        >
          <Settings className="size-4 shrink-0" strokeWidth={1.75} />
          {!collapsed ? <span>{t.nav.settings}</span> : null}
        </NavLink>
        <div
          className={[
            "flex items-center text-xs text-rmb-muted",
            collapsed ? "h-8 justify-center px-0" : "h-8 gap-2.5 px-2.5",
          ].join(" ")}
          title={collapsed ? daemonLabel : undefined}
        >
          <span
            className={[
              "size-1.5 shrink-0 rounded-full",
              daemonUp === null ? "bg-rmb-faint" : daemonUp ? "bg-rmb-accent" : "bg-rmb-danger",
            ].join(" ")}
            aria-hidden
          />
          {!collapsed ? <span className="truncate">{daemonLabel}</span> : null}
          {collapsed ? (
            <span className="sr-only">{daemonLabel}</span>
          ) : null}
        </div>
      </div>
      </aside>
      <button
        type="button"
        onClick={toggle}
        aria-expanded={!collapsed}
        aria-label={toggleLabel}
        title={toggleLabel}
        className="absolute right-0 top-1/2 z-10 flex size-6 -translate-y-1/2 translate-x-1/2 items-center justify-center rounded-full border border-rmb-line bg-white text-rmb-muted shadow-sm transition-colors hover:bg-rmb-fill hover:text-rmb-dark"
      >
        <ToggleIcon className="size-3.5" strokeWidth={1.75} aria-hidden />
      </button>
    </div>
  );
}
