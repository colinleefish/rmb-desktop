import { useState } from "react";
import { Check, ChevronDown, ChevronUp, FileCode2 } from "lucide-react";
import type { SetupArtifact } from "../../lib/agentSetupTypes";
import { useI18n } from "../../i18n";

function fileActionLabel(
  artifact: SetupArtifact,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (artifact.changeType === "unchanged") {
    return t.agents.fileReplace.upToDate;
  }
  if (artifact.exists) {
    return t.agents.fileReplace.willReplace;
  }
  return t.agents.fileReplace.willCreate;
}

function applyButtonLabel(
  artifact: SetupArtifact,
  applied: boolean,
  applying: boolean,
  t: ReturnType<typeof useI18n>["t"],
): string {
  if (applied || artifact.changeType === "unchanged") {
    return t.agents.applied;
  }
  if (applying) {
    return t.common.loading;
  }
  if (artifact.exists) {
    return t.agents.fileReplace.replaceFile;
  }
  return t.agents.fileReplace.installFile;
}

export function FileReplaceReview({
  artifact,
  title,
  applied,
  applying = false,
  onApply,
}: {
  artifact: SetupArtifact;
  title?: string;
  applied: boolean;
  applying?: boolean;
  onApply: () => void;
}) {
  const { t } = useI18n();
  const [previewOpen, setPreviewOpen] = useState(false);

  const upToDate = artifact.changeType === "unchanged";
  const actionLabel = fileActionLabel(artifact, t);

  return (
    <div className="overflow-hidden rounded-xl border border-rmb-gray/35 bg-white shadow-sm">
      <div className="border-b border-rmb-gray/10 px-5 py-4">
        <div className="flex flex-wrap items-start gap-3">
          <FileCode2 className="mt-0.5 size-4 shrink-0 stroke-rmb-accent" />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-sm font-semibold text-rmb-dark">{title ?? artifact.title}</h3>
              {(applied || upToDate) && (
                <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 ring-1 ring-emerald-200">
                  <Check className="size-3" />
                  {t.agents.applied}
                </span>
              )}
            </div>
            <p className="mt-1 font-mono text-xs text-rmb-gray">{artifact.path}</p>
            <p className="mt-2 text-sm text-rmb-gray">{artifact.description}</p>
          </div>
          <span
            className={[
              "rounded-full px-2.5 py-1 text-[11px] font-medium ring-1",
              upToDate
                ? "bg-emerald-50 text-emerald-800 ring-emerald-200"
                : artifact.exists
                  ? "bg-amber-50 text-amber-900 ring-amber-200"
                  : "bg-sky-50 text-sky-900 ring-sky-200",
            ].join(" ")}
          >
            {actionLabel}
          </span>
        </div>
      </div>

      {artifact.warnings.length > 0 && (
        <div className="space-y-1 border-b border-amber-100 bg-amber-50/60 px-5 py-3">
          {artifact.warnings.map((w) => (
            <p key={w} className="text-xs text-amber-900">
              {w}
            </p>
          ))}
        </div>
      )}

      <div className="border-b border-rmb-gray/10 px-5 py-3">
        <button
          type="button"
          onClick={() => setPreviewOpen((open) => !open)}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-rmb-accent hover:text-rmb-accent/80"
        >
          {previewOpen ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
          {previewOpen ? t.agents.fileReplace.hidePreview : t.agents.fileReplace.previewFile}
        </button>
        {previewOpen && (
          <div className="mt-3 overflow-hidden rounded-lg border border-rmb-gray/15 bg-[#fafafa]">
            <pre className="m-0 max-h-[28rem] overflow-auto p-3 font-mono text-[12px] leading-5 text-rmb-dark/90">
              {artifact.proposed || "\u00a0"}
            </pre>
          </div>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-start gap-2 bg-rmb-light/30 px-5 py-3">
        <button
          type="button"
          onClick={onApply}
          disabled={applied || applying || upToDate}
          className="rounded-md bg-rmb-accent px-4 py-1.5 text-sm font-medium text-white hover:bg-rmb-accent/90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {applyButtonLabel(artifact, applied, applying, t)}
        </button>
      </div>
    </div>
  );
}
