import { useMemo, useState } from "react";
import { Check, Copy, FileCode2, Rows2, SplitSquareHorizontal } from "lucide-react";
import type { SetupArtifact } from "../../lib/agentSetupTypes";
import { useI18n } from "../../i18n";

type DiffLine = {
  text: string;
  kind: "same" | "add" | "remove" | "change";
};

function computeDiff(current: string, proposed: string): DiffLine[] {
  const left = current.split("\n");
  const right = proposed.split("\n");
  const max = Math.max(left.length, right.length);
  const lines: DiffLine[] = [];

  for (let i = 0; i < max; i++) {
    const l = left[i];
    const r = right[i];
    if (l === undefined && r !== undefined) {
      lines.push({ text: r, kind: "add" });
    } else if (l !== undefined && r === undefined) {
      lines.push({ text: l, kind: "remove" });
    } else if (l === r) {
      lines.push({ text: l ?? "", kind: "same" });
    } else {
      if (l) lines.push({ text: l, kind: "remove" });
      if (r) lines.push({ text: r, kind: "add" });
    }
  }
  return lines;
}

function lineClass(kind: DiffLine["kind"]) {
  switch (kind) {
    case "add":
      return "bg-emerald-50/80 text-emerald-900";
    case "remove":
      return "bg-red-50/80 text-red-900 line-through decoration-red-300/60";
    case "change":
      return "bg-amber-50/80 text-amber-900";
    default:
      return "text-rmb-dark/90";
  }
}

function UnifiedDiff({ lines }: { lines: DiffLine[] }) {
  return (
    <div className="overflow-hidden rounded-lg border border-rmb-gray/15 bg-[#fafafa]">
      <pre className="m-0 max-h-[28rem] overflow-auto p-3 font-mono text-[12px] leading-5">
        {lines.map((line, i) => (
          <div key={i} className={`whitespace-pre-wrap break-all px-1 ${lineClass(line.kind)}`}>
            <span className="mr-2 inline-block w-4 select-none text-rmb-gray/40">
              {line.kind === "add" ? "+" : line.kind === "remove" ? "−" : " "}
            </span>
            {line.text || "\u00a0"}
          </div>
        ))}
      </pre>
    </div>
  );
}

function SideBySideDiff({
  current,
  proposed,
  currentLabel,
  proposedLabel,
}: {
  current: string;
  proposed: string;
  currentLabel: string;
  proposedLabel: string;
}) {
  const currentLines = current.split("\n");
  const proposedLines = proposed.split("\n");
  const max = Math.max(currentLines.length, proposedLines.length);

  return (
    <div className="grid min-h-[20rem] gap-3 lg:grid-cols-2">
      <div className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-rmb-gray/15 bg-[#fafafa]">
        <div className="border-b border-rmb-gray/10 bg-white px-3 py-2 text-xs font-medium text-rmb-gray">
          {currentLabel}
        </div>
        <pre className="m-0 flex-1 overflow-auto p-3 font-mono text-[12px] leading-5">
          {Array.from({ length: max }, (_, i) => {
            const line = currentLines[i];
            const other = proposedLines[i];
            const changed = line !== undefined && other !== undefined && line !== other;
            const removed = line !== undefined && other === undefined;
            return (
              <div
                key={i}
                className={[
                  "whitespace-pre-wrap break-all px-0.5",
                  changed || removed ? "bg-red-50/70 text-red-900" : "text-rmb-dark/90",
                ].join(" ")}
              >
                {line ?? ""}
              </div>
            );
          })}
        </pre>
      </div>
      <div className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-rmb-gray/15 bg-[#fafafa]">
        <div className="border-b border-rmb-gray/10 bg-white px-3 py-2 text-xs font-medium text-rmb-gray">
          {proposedLabel}
        </div>
        <pre className="m-0 flex-1 overflow-auto p-3 font-mono text-[12px] leading-5">
          {Array.from({ length: max }, (_, i) => {
            const line = proposedLines[i];
            const other = currentLines[i];
            const changed = line !== undefined && other !== undefined && line !== other;
            const added = line !== undefined && other === undefined;
            return (
              <div
                key={i}
                className={[
                  "whitespace-pre-wrap break-all px-0.5",
                  changed || added ? "bg-emerald-50/80 text-emerald-900" : "text-rmb-dark/90",
                ].join(" ")}
              >
                {line ?? ""}
              </div>
            );
          })}
        </pre>
      </div>
    </div>
  );
}

export function ConfigDiffReview({
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
  const [view, setView] = useState<"split" | "unified">(
    artifact.language === "json" ? "split" : "unified",
  );
  const [copied, setCopied] = useState(false);

  const diffLines = useMemo(
    () => computeDiff(artifact.current, artifact.proposed),
    [artifact.current, artifact.proposed],
  );

  const added = diffLines.filter((l) => l.kind === "add").length;
  const removed = diffLines.filter((l) => l.kind === "remove").length;

  async function handleCopy() {
    await navigator.clipboard.writeText(artifact.proposed);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="overflow-hidden rounded-xl border border-rmb-gray/35 bg-white shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-rmb-gray/10 px-5 py-4">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <FileCode2 className="size-4 shrink-0 stroke-rmb-accent" />
            <h3 className="text-sm font-semibold text-rmb-dark">{title ?? artifact.title}</h3>
            {applied && (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 ring-1 ring-emerald-200">
                <Check className="size-3" />
                {t.agents.applied}
              </span>
            )}
          </div>
          <p className="mt-1 font-mono text-xs text-rmb-gray">{artifact.path}</p>
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-rmb-gray/15 bg-rmb-light/50 p-0.5">
          <button
            type="button"
            onClick={() => setView("split")}
            className={[
              "inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs transition",
              view === "split" ? "bg-white text-rmb-dark shadow-sm" : "text-rmb-gray hover:text-rmb-dark",
            ].join(" ")}
          >
            <SplitSquareHorizontal className="size-3.5" />
            {t.agents.viewSplit}
          </button>
          <button
            type="button"
            onClick={() => setView("unified")}
            className={[
              "inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs transition",
              view === "unified" ? "bg-white text-rmb-dark shadow-sm" : "text-rmb-gray hover:text-rmb-dark",
            ].join(" ")}
          >
            <Rows2 className="size-3.5" />
            {t.agents.viewUnified}
          </button>
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

      <div className="px-5 py-4">
        {!applied && (added > 0 || removed > 0) && (
          <p className="mb-3 text-xs text-rmb-gray">
            <span className="text-emerald-700">+{added}</span>
            {removed > 0 && (
              <>
                {" "}
                <span className="text-red-600">−{removed}</span>
              </>
            )}{" "}
            {t.agents.linesChanged}
          </p>
        )}

        {view === "split" ? (
          <SideBySideDiff
            current={applied ? artifact.proposed : artifact.current}
            proposed={artifact.proposed}
            currentLabel={t.agents.current}
            proposedLabel={applied ? t.agents.current : t.agents.afterApply}
          />
        ) : (
          <UnifiedDiff
            lines={
              applied
                ? artifact.proposed.split("\n").map((text) => ({ text, kind: "same" as const }))
                : diffLines
            }
          />
        )}
      </div>

      <div className="flex flex-wrap items-center justify-start gap-2 border-t border-rmb-gray/15 bg-rmb-light/30 px-5 py-3">
        {artifact.applyMode === "copy_only" ? (
          <button
            type="button"
            onClick={handleCopy}
            className="inline-flex items-center gap-1.5 rounded-md border border-rmb-gray/20 bg-white px-3 py-1.5 text-sm font-medium text-rmb-dark hover:bg-rmb-light"
          >
            {copied ? <Check className="size-4 text-emerald-600" /> : <Copy className="size-4" />}
            {copied ? t.agents.copied : t.agents.copyProposed}
          </button>
        ) : (
          <button
            type="button"
            onClick={onApply}
            disabled={applied || applying || artifact.changeType === "unchanged"}
            className="rounded-md bg-rmb-accent px-4 py-1.5 text-sm font-medium text-white hover:bg-rmb-accent/90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {applied ? t.agents.applied : applying ? t.common.loading : t.agents.applyChange}
          </button>
        )}
      </div>
    </div>
  );
}
