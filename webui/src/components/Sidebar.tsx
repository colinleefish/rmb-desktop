import { NavLink, useLocation } from "react-router-dom";
import type { ComponentType } from "react";
import {
  LayoutDashboard,
  MessagesSquare,
  Settings,
  Sparkles,
  Wand2,
} from "lucide-react";
import { useEffect, useState } from "react";
import { getOverview } from "../lib/api";
import {
  MEMORY_CATEGORIES,
  type MemoryCategory,
} from "../lib/memoryCategories";
import type { MemoryCategoryOverview, OverviewCounts } from "../lib/types";
import { useI18n } from "../i18n";
import type { Lang } from "../i18n/translations";

type NavChild = {
  to: string;
  label: string;
  category?: MemoryCategory;
};

type NavItem = {
  to: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
  countKey?: keyof OverviewCounts;
  end?: boolean;
  disabled?: boolean;
  children?: NavChild[];
};

const navLinkClass = (isActive: boolean) =>
  [
    "flex items-center gap-2 rounded-md px-2 py-2 text-sm transition",
    isActive
      ? "bg-rmb-accent text-white [&_svg]:stroke-white"
      : "text-rmb-gray hover:bg-rmb-light [&_svg]:stroke-rmb-gray",
  ].join(" ");

function memoryCategoryBadge(
  category: MemoryCategory,
  stats: MemoryCategoryOverview | null,
): string | undefined {
  if (!stats) return undefined;
  if (category === "profile") {
    return stats.profile_version > 0 ? `v${stats.profile_version}` : undefined;
  }
  const count = stats[category];
  return String(count);
}

function SidebarNavChildren({
  items,
  memoryByCategory,
}: {
  items: NavChild[];
  memoryByCategory: MemoryCategoryOverview | null;
}) {
  const location = useLocation();

  return (
    <ul className="ml-5 mt-0.5 space-y-0.5 border-l border-rmb-gray/15 pl-2">
      {items.map((child) => {
        const isActive = location.pathname === child.to;
        const badge = child.category
          ? memoryCategoryBadge(child.category, memoryByCategory)
          : undefined;
        return (
          <li key={child.to}>
            <NavLink
              to={child.to}
              className={[
                "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition",
                isActive
                  ? "bg-rmb-accent/10 font-medium text-rmb-accent"
                  : "text-rmb-gray hover:bg-rmb-light hover:text-rmb-dark",
              ].join(" ")}
            >
              <span className="flex-1">{child.label}</span>
              {badge !== undefined && (
                <span
                  className={[
                    "rounded px-1.5 py-0.5 text-[10px] tabular-nums",
                    isActive
                      ? "bg-rmb-accent/15 text-rmb-accent"
                      : "bg-rmb-light text-rmb-gray",
                  ].join(" ")}
                >
                  {badge}
                </span>
              )}
            </NavLink>
          </li>
        );
      })}
    </ul>
  );
}

function SidebarNavItem({
  item,
  counts,
  memoryByCategory,
  soonLabel,
}: {
  item: NavItem;
  counts: OverviewCounts | null;
  memoryByCategory: MemoryCategoryOverview | null;
  soonLabel: string;
}) {
  const Icon = item.icon;

  if (item.disabled) {
    return (
      <span className="flex items-center gap-2 rounded-md px-2 py-2 text-sm text-rmb-gray/60 opacity-60">
        <Icon className="size-4 shrink-0 stroke-rmb-gray/60" />
        <span className="flex-1">{item.label}</span>
        <span className="text-[10px] uppercase">{soonLabel}</span>
      </span>
    );
  }

  if (item.children?.length) {
    const location = useLocation();
    const sectionActive = item.children.some((child) => location.pathname === child.to);

    return (
      <div>
        <div
          className={[
            "flex items-center gap-2 rounded-md px-2 py-2 text-sm",
            sectionActive ? "font-medium text-rmb-dark" : "text-rmb-gray",
          ].join(" ")}
        >
          <Icon
            className={[
              "size-4 shrink-0",
              sectionActive ? "stroke-rmb-accent" : "stroke-rmb-gray",
            ].join(" ")}
          />
          <span className="flex-1">{item.label}</span>
        </div>
        <SidebarNavChildren items={item.children} memoryByCategory={memoryByCategory} />
      </div>
    );
  }

  return (
    <NavLink to={item.to} end={item.end} className={({ isActive }) => navLinkClass(isActive)}>
      {({ isActive }) => (
        <>
          <Icon className="size-4 shrink-0" />
          <span className="flex-1">{item.label}</span>
          {item.countKey && counts && (
            <span
              className={
                isActive
                  ? "rounded bg-rmb-dark/30 px-1.5 py-0.5 text-[10px] tabular-nums text-white"
                  : "rounded bg-rmb-light px-1.5 py-0.5 text-[10px] tabular-nums text-rmb-gray"
              }
            >
              {counts[item.countKey]}
            </span>
          )}
        </>
      )}
    </NavLink>
  );
}

export function Sidebar() {
  const { t, lang, setLang } = useI18n();
  const location = useLocation();
  const [counts, setCounts] = useState<OverviewCounts | null>(null);
  const [memoryByCategory, setMemoryByCategory] = useState<MemoryCategoryOverview | null>(null);

  useEffect(() => {
    getOverview()
      .then((o) => {
        setCounts(o.counts);
        setMemoryByCategory(o.memory_by_category);
      })
      .catch(() => {});
  }, []);

  const groups: { label: string; items: NavItem[] }[] = [
    {
      label: t.nav.home,
      items: [
        { to: "/", label: t.nav.overview, icon: LayoutDashboard, end: true },
      ],
    },
    {
      label: t.nav.perSession,
      items: [
        {
          to: "/sessions",
          label: t.nav.sessions,
          icon: MessagesSquare,
          countKey: "sessions",
        },
      ],
    },
    {
      label: t.nav.acrossSessions,
      items: [
        {
          to: "/memories/profile",
          label: t.nav.memories,
          icon: Sparkles,
          children: MEMORY_CATEGORIES.map((category) => ({
            to: `/memories/${category}`,
            label: t.memories.categories[category].nav,
            category,
          })),
        },
        {
          to: "/skills",
          label: t.nav.skills,
          icon: Wand2,
          countKey: "skills",
        },
      ],
    },
  ];

  return (
    <aside className="flex h-full w-60 shrink-0 flex-col border-r border-rmb-gray/20 bg-white">
      <div className="flex items-center gap-3 border-b border-rmb-gray/15 px-4 py-4">
        <img
          src={`${import.meta.env.BASE_URL}logo.svg`}
          alt=""
          className="size-9 shrink-0 rounded-lg"
          width={36}
          height={36}
        />
        <div className="leading-tight">
          <div className="text-sm font-semibold text-rmb-dark">{t.appName}</div>
          <div className="text-xs text-rmb-gray">{t.appSubtitle}</div>
        </div>
      </div>

      <nav className="flex-1 overflow-y-auto px-2 py-3">
        {groups.map((group) => (
          <div key={group.label} className="mb-4">
            <div className="px-2 pb-1 text-[11px] font-semibold uppercase tracking-wide text-rmb-gray/60">
              {group.label}
            </div>
            <ul className="space-y-0.5">
              {group.items.map((item) => (
                <li key={item.to}>
                  <SidebarNavItem
                    item={item}
                    counts={counts}
                    memoryByCategory={memoryByCategory}
                    soonLabel={t.nav.soon}
                  />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      <div className="space-y-2 border-t border-rmb-gray/15 p-3">
        <label className="block px-1 text-[11px] font-medium text-rmb-gray">
          {t.settings.language.label}
        </label>
        <select
          value={lang}
          onChange={(e) => setLang(e.target.value as Lang)}
          className="w-full rounded-md border border-rmb-gray/20 bg-white px-2 py-1.5 text-sm text-rmb-dark"
        >
          <option value="en">{t.settings.language.en}</option>
          <option value="zh">{t.settings.language.zh}</option>
        </select>

        <NavLink
          to="/settings/general"
          className={({ isActive }) =>
            navLinkClass(isActive || location.pathname.startsWith("/settings"))
          }
        >
          <Settings className="size-4 shrink-0" />
          {t.nav.settings}
        </NavLink>
      </div>
    </aside>
  );
}
