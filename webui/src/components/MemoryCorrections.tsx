import { useCallback, useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import {
  createCorrection,
  listCorrections,
  retractCorrection,
} from "../lib/api";
import type { CorrectionRow } from "../lib/types";
import { formatDateTime, formatDateTimeMonoClass } from "../lib/format";
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
    <div className="mt-6 space-y-4 border-t border-rmb-line pt-5">
      <div>
        <h3 className="text-sm font-semibold text-rmb-dark">
          {t.memories.corrections.title}{" "}
          <span className="font-normal text-rmb-faint">{rows.length}</span>
        </h3>
        {loading ? (
          <p className="mt-2 text-sm text-rmb-muted">{t.memories.loading}</p>
        ) : error ? (
          <p className="mt-2 text-sm text-rmb-danger">{error}</p>
        ) : rows.length === 0 ? (
          <p className="mt-2 text-sm text-rmb-muted">{t.memories.corrections.empty}</p>
        ) : (
          <div className="mt-2 divide-y divide-rmb-line rounded-md border border-rmb-line">
            {rows.map((row) => (
              <div key={row.uri} className="flex items-start justify-between gap-3 px-3 py-2.5">
                <div className="min-w-0">
                  <p className="whitespace-pre-wrap text-sm text-rmb-dark">{row.statement}</p>
                  <p className={`mt-1 text-xs text-rmb-faint ${formatDateTimeMonoClass}`}>
                    {formatDateTime(row.created_at)}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => handleRetract(row.uri)}
                  disabled={retractingURI === row.uri}
                  className="shrink-0 rounded-md p-1 text-rmb-faint transition-colors hover:bg-rmb-fill hover:text-rmb-danger disabled:opacity-50"
                  aria-label={t.memories.corrections.retract}
                >
                  <Trash2 className="size-4" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <div>
        <h3 className="text-sm font-semibold text-rmb-dark">{t.memories.corrections.addTitle}</h3>
        <textarea
          value={statement}
          onChange={(e) => setStatement(e.target.value)}
          placeholder={t.memories.corrections.placeholder}
          rows={3}
          className="mt-2 w-full resize-y rounded-md border border-rmb-line-strong px-3 py-2 text-sm text-rmb-dark outline-none transition-colors placeholder:text-rmb-faint focus:border-rmb-accent"
        />
        {submitError && <p className="mt-2 text-sm text-rmb-danger">{submitError}</p>}
        <div className="mt-2 flex justify-end">
          <button
            type="button"
            onClick={handleAdd}
            disabled={submitting || statement.trim() === ""}
            className="h-8 rounded-md bg-rmb-accent px-3 text-sm font-medium text-white transition-colors hover:bg-rmb-accent/90 active:scale-[0.98] disabled:opacity-50"
          >
            {submitting ? t.memories.corrections.adding : t.memories.corrections.add}
          </button>
        </div>
      </div>
    </div>
  );
}
