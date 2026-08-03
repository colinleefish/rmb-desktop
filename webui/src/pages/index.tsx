import { useEffect, useState } from "react";
import { getOverview } from "../lib/api";
import type { OverviewCounts } from "../lib/types";
import { OverviewPage } from "./OverviewPage";
import { useI18n } from "../i18n";

export function OverviewRoute() {
  const { t } = useI18n();
  const [counts, setCounts] = useState<OverviewCounts | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getOverview()
      .then((data) => setCounts(data.counts))
      .catch((err: Error) => setError(err.message));
  }, []);

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
        {t.common.error}: {error}
      </div>
    );
  }
  if (!counts) {
    return <p className="text-rmb-gray">{t.common.loading}</p>;
  }
  return <OverviewPage counts={counts} />;
}
