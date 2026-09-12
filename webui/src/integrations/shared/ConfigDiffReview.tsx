import { useMemo, useState } from "react";
import { Check, Copy, FileCode2, Rows2, SplitSquareHorizontal } from "lucide-react";
import type { SetupArtifact } from "../../lib/agentSetupTypes";
import { useI18n } from "../../i18n";
import { StatusPill } from "../../components/StatusPill";

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
      return "bg-rmb-accent/10 text-rmb-dark";
    case "remove":
      return "bg-rmb-danger-soft text-rmb-danger/90 line-through decoration-rmb-danger/30";
    case "change":
      return "bg-rmb-warn-soft text-rmb-dark";
    default:
      return "text-rmb-dark/90";
  }
}

function UnifiedDiff({ lines }: { lines: DiffLine[] }) {
  return (
    <div className="overflow-hidden rounded-md border border-rmb-line bg-rmb-fill">
      <pre className="m-0 max-h-[28rem] overflow-auto p-3 font-mono text-[12px] leading-5">
        {lines.map((line, i) => (
          <div key={i} className={`whitespace-pre-wrap break-all px-1 ${lineClass(line.kind)}`}>
            <span className="mr-2 inline-block w-4 select-none text-rmb-faint">
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
      <div className="flex min-h-0 flex-col overflow-hidden rounded-md border border-rmb-line bg-rmb-fill">
        <div className="border-b border-rmb-line bg-white px-3 py-2 text-xs font-medium text-rmb-muted">
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
                  changed || removed ? "bg-rmb-danger-soft text-rmb-danger/90" : "text-rmb-dark/90",
                ].join(" ")}
              >
                {line ?? ""}
              </div>
            );
          })}
        </pre>
      </div>
      <div className="flex min-h-0 flex-col overflow-hidden rounded-md border border-rmb-line bg-rmb-fill">
        <div className="border-b border-rmb-line bg-white px-3 py-2 text-xs font-medium text-rmb-muted">
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
                  changed || added ? "bg-rmb-accent/10 text-rmb-dark" : "text-rmb-dark/90",
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

  const { current, proposed } = artifact;
  const diffLines = useMemo(
    () => computeDiff(current, proposed),
    [current, proposed],
  );

  const added = diffLines.filter((l) => l.kind === "add").length;
  const removed = diffLines.filter((l) => l.kind === "remove").length;

  async function handleCopy() {
    await navigator.clipboard.writeText(artifact.proposed);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="overflow-hidden rounded-md border border-rmb-line bg-white">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-rmb-line px-5 py-4">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <FileCode2 className="size-4 shrink-0 stroke-rmb-accent" />
            <h3 className="text-sm font-semibold text-rmb-dark">{title ?? artifact.title}</h3>
            {applied && (
              <StatusPill tone="ok">
                <Check className="mr-0.5 size-3" />
                {t.agents.applied}
              </StatusPill>
            )}
          </div>
          <p className="mt-1 font-mono text-xs text-rmb-muted">{artifact.path}</p>
        </div>
        <div className="flex items-center gap-0.5 rounded-md border border-rmb-line bg-rmb-fill p-0.5">
          <button
            type="button"
            onClick={() => setView("split")}
            className={[
              "inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs transition-colors",
              view === "split"
                ? "bg-white font-medium text-rmb-dark"
                : "text-rmb-muted hover:text-rmb-dark",
            ].join(" ")}
          >
            <SplitSquareHorizontal className="size-3.5" />
            {t.agents.viewSplit}
          </button>
          <button
            type="button"
            onClick={() => setView("unified")}
            className={[
              "inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs transition-colors",
              view === "unified"
                ? "bg-white font-medium text-rmb-dark"
                : "text-rmb-muted hover:text-rmb-dark",
            ].join(" ")}
          >
            <Rows2 className="size-3.5" />
            {t.agents.viewUnified}
          </button>
        </div>
      </div>

      {artifact.warnings.length > 0 && (
        <div className="space-y-1 border-b border-rmb-line bg-rmb-warn-soft px-5 py-3">
          {artifact.warnings.map((w) => (
            <p key={w} className="text-xs text-rmb-warn">
              {w}
            </p>
          ))}
        </div>
      )}

      <div className="px-5 py-4">
        {!applied && (added > 0 || removed > 0) && (
          <p className="mb-3 text-xs text-rmb-muted">
            <span className="text-rmb-accent">+{added}</span>
            {removed > 0 && (
              <>
                {" "}
                <span className="text-rmb-danger">−{removed}</span>
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

      <div className="flex flex-wrap items-center justify-start gap-2 border-t border-rmb-line bg-rmb-fill px-5 py-3">
        {artifact.applyMode === "copy_only" ? (
          <button
            type="button"
            onClick={handleCopy}
            className="inline-flex items-center gap-1.5 rounded-md border border-rmb-line-strong bg-white px-3 py-1.5 text-sm font-medium text-rmb-dark hover:bg-rmb-fill"
          >
            {copied ? <Check className="size-4 text-rmb-accent" /> : <Copy className="size-4" />}
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
