import { useEffect, useState } from "react";
import { getPipelineHealth, pageSessions } from "../lib/api";
import { useSharedOverview } from "../lib/overviewCounts";
import type { PipelineHealth, SessionRow } from "../lib/types";
import { ErrorNote, ListSkeleton } from "../components/EmptyState";
import { OverviewPage } from "./OverviewPage";
import { useI18n } from "../i18n";

export function OverviewRoute() {
  const { t } = useI18n();
  const overview = useSharedOverview();
  const [health, setHealth] = useState<PipelineHealth | null>(null);
  const [recent, setRecent] = useState<SessionRow[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      getPipelineHealth(),
      pageSessions({ limit: 5, offset: 0, sort: "updated", order: "desc" }).catch(() => null),
    ])
      .then(([pipelineHealth, sessions]) => {
        if (cancelled) return;
        setHealth(pipelineHealth);
        setRecent(sessions?.items ?? []);
      })
      .catch((err: Error) => {
        if (!cancelled) setError(err.message);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <ErrorNote>
        {t.common.error}: {error}
      </ErrorNote>
    );
  }
  if (!overview || !health) {
    return <ListSkeleton rows={5} />;
  }
  return <OverviewPage overview={overview} health={health} recent={recent} />;
}
