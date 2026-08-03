import { useCallback, useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import {
  createCorrection,
  listCorrections,
  retractCorrection,
} from "../lib/api";
import type { CorrectionRow } from "../lib/types";
import { formatDateTime } from "../lib/format";
import { useI18n } from "../i18n";

export function MemoryCorrections({ memoryURI }: { memoryURI: string }) {
  const { t } = useI18n();
  const [rows, setRows] = useState<CorrectionRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [statement, setStatement] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [retractingURI, setRetractingURI] = useState<string | null>(null);

  const reload = useCallback(() => {
    setLoading(true);
    setError(null);
    listCorrections(memoryURI)
      .then(setRows)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [memoryURI]);

  useEffect(() => {
    setRows([]);
    setSubmitError(null);
    setStatement("");
    reload();
  }, [memoryURI, reload]);

  const handleAdd = () => {
    const trimmed = statement.trim();
    if (!trimmed) return;
    setSubmitting(true);
    setSubmitError(null);
    createCorrection({ statement: trimmed, target_uris: [memoryURI] })
      .then(() => {
        setStatement("");
        reload();
      })
      .catch((err: Error) => setSubmitError(err.message))
      .finally(() => setSubmitting(false));
  };

  const handleRetract = (uri: string) => {
    setRetractingURI(uri);
    retractCorrection(uri)
      .then(() => reload())
      .catch((err: Error) => setError(err.message))
      .finally(() => setRetractingURI(null));
  };

  return (
    <div className="space-y-4 border-t border-rmb-gray/15 pt-4">
      <div>
        <h3 className="text-xs font-semibold uppercase tracking-wide text-rmb-gray">
          {t.memories.corrections.title} ({rows.length})
        </h3>
        {loading ? (
          <p className="mt-2 text-sm text-rmb-gray">{t.memories.loading}</p>
        ) : error ? (
          <p className="mt-2 text-sm text-red-600">{error}</p>
        ) : rows.length === 0 ? (
          <p className="mt-2 text-sm text-rmb-gray">{t.memories.corrections.empty}</p>
        ) : (
          <div className="mt-2 space-y-2">
            {rows.map((row) => (
              <div
                key={row.uri}
                className="rounded-lg border border-rmb-gray/15 bg-rmb-light/40 p-3"
              >
                <div className="flex items-start justify-between gap-3">
                  <p className="whitespace-pre-wrap text-sm text-rmb-dark">{row.statement}</p>
                  <button
                    type="button"
                    onClick={() => handleRetract(row.uri)}
                    disabled={retractingURI === row.uri}
                    className="shrink-0 rounded p-1 text-rmb-gray transition hover:bg-white hover:text-red-600 disabled:opacity-50"
                    aria-label={t.memories.corrections.retract}
                  >
                    <Trash2 className="size-4" />
                  </button>
                </div>
                <p className="mt-2 text-xs text-rmb-gray">{formatDateTime(row.created_at)}</p>
              </div>
            ))}
          </div>
        )}
      </div>

      <div>
        <h3 className="text-xs font-semibold uppercase tracking-wide text-rmb-gray">
          {t.memories.corrections.addTitle}
        </h3>
        <textarea
          value={statement}
          onChange={(e) => setStatement(e.target.value)}
          placeholder={t.memories.corrections.placeholder}
          rows={3}
          className="mt-2 w-full resize-y rounded-md border border-rmb-gray/20 px-3 py-2 text-sm text-rmb-dark outline-none focus:border-rmb-accent"
        />
        {submitError && <p className="mt-2 text-sm text-red-600">{submitError}</p>}
        <div className="mt-2 flex justify-end">
          <button
            type="button"
            onClick={handleAdd}
            disabled={submitting || statement.trim() === ""}
            className="rounded-md bg-rmb-accent px-3 py-1.5 text-sm font-medium text-white transition hover:bg-rmb-accent/90 disabled:opacity-50"
          >
            {submitting ? t.memories.corrections.adding : t.memories.corrections.add}
          </button>
        </div>
      </div>
    </div>
  );
}
