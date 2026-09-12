import { Link } from "react-router-dom";
import { memoryBrowsePath } from "../lib/memoryBrowsePath";
import { useI18n } from "../i18n";

export function CorrectionTargetURIs({ uris }: { uris?: string[] }) {
  const { t } = useI18n();
  const targets = (uris ?? []).map((u) => u.trim()).filter((u) => u !== "");

  if (targets.length === 0) {
    return (
      <span
        className="font-mono text-xs text-rmb-faint"
        aria-label={t.memories.corrections.noTargetUri}
      >
        —
      </span>
    );
  }

  return (
    <div className="space-y-0.5">
      {targets.map((uri) => {
        const to = memoryBrowsePath(uri);
        const className = "block truncate font-mono text-xs text-rmb-faint";
        if (to) {
          return (
            <Link
              key={uri}
              to={to}
              className={`${className} hover:text-rmb-accent`}
              title={uri}
            >
              {uri}
            </Link>
          );
        }
        return (
          <span key={uri} className={className} title={uri}>
            {uri}
          </span>
        );
      })}
    </div>
  );
}
