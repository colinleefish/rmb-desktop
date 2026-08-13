import { useEffect, useState } from "react";
import { getOverview, getPipelineHealth } from "../lib/api";
import type { OverviewCounts, PipelineHealth } from "../lib/types";
import { OverviewPage } from "./OverviewPage";
import { useI18n } from "../i18n";

export function OverviewRoute() {
  const { t } = useI18n();
  const [counts, setCounts] = useState<OverviewCounts | null>(null);
  const [health, setHealth] = useState<PipelineHealth | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([getOverview(), getPipelineHealth()])
      .then(([overview, pipelineHealth]) => {
        if (cancelled) return;
        setCounts(overview.counts);
        setHealth(pipelineHealth);
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
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
        {t.common.error}: {error}
      </div>
    );
  }
  if (!counts || !health) {
    return <p className="text-rmb-gray">{t.common.loading}</p>;
  }
  return <OverviewPage counts={counts} health={health} />;
}
