import { NavLink, type To } from "react-router-dom";
import { tabClass } from "../lib/tabClass";

export type PageTab = {
  to: To;
  label: string;
  end?: boolean;
  badge?: string | number;
};

export function PageTabs({ tabs }: { tabs: PageTab[] }) {
  return (
    <div className="flex flex-wrap gap-1 border-b border-rmb-line">
      {tabs.map((tab) => (
        <NavLink
          key={typeof tab.to === "string" ? tab.to : JSON.stringify(tab.to)}
          to={tab.to}
          end={tab.end}
          className={({ isActive }) => tabClass(isActive)}
        >
          {tab.label}
          {tab.badge !== undefined && (
            <span className="text-[11px] text-rmb-faint">{tab.badge}</span>
          )}
        </NavLink>
      ))}
    </div>
  );
}
