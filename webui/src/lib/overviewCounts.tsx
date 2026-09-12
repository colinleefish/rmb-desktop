import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { getOverview } from "./api";
import type { Overview, OverviewCounts } from "./types";

let sharedOverviewLoad: Promise<Overview> | null = null;

function loadSharedOverview(): Promise<Overview> {
  if (!sharedOverviewLoad) {
    sharedOverviewLoad = getOverview().catch((err) => {
      sharedOverviewLoad = null;
      throw err;
    });
  }
  return sharedOverviewLoad;
}

type OverviewDataContextValue = {
  overview: Overview | null;
  counts: OverviewCounts | null;
};

const OverviewDataContext = createContext<OverviewDataContextValue>({
  overview: null,
  counts: null,
});

export function OverviewCountsProvider({ children }: { children: ReactNode }) {
  const [value, setValue] = useState<OverviewDataContextValue>({
    overview: null,
    counts: null,
  });

  useEffect(() => {
    loadSharedOverview()
      .then((overview) => setValue({ overview, counts: overview.counts }))
      .catch(() => {});
  }, []);

  return (
    <OverviewDataContext.Provider value={value}>{children}</OverviewDataContext.Provider>
  );
}

export function useOverviewCounts(): OverviewCounts | null {
  return useContext(OverviewDataContext).counts;
}

export function useSharedOverview(): Overview | null {
  return useContext(OverviewDataContext).overview;
}
