import { INTEGRATIONS } from "../integrations/registry";
import { AgentLogo } from "../integrations/shared/AgentLogo";
import { sessionSourceLabel } from "../lib/format";

function resolveIntegration(source: string | null | undefined) {
  const key = (source ?? "").toLowerCase();
  const id = key === "cc" || key === "claude" ? "claude-code" : key;
  return INTEGRATIONS.find((entry) => entry.id === id);
}

/** Agent identity: logo + label, resolved from a session `source` or an integration id. */
export function AgentChip({
  source,
  size = 16,
  className = "",
}: {
  source: string | null | undefined;
  size?: number;
  className?: string;
}) {
  const integration = resolveIntegration(source);
  const label = integration?.label ?? sessionSourceLabel(source);
  return (
    <span
      className={`inline-flex min-w-0 items-center gap-1.5 text-xs text-rmb-muted ${className}`}
      title={label}
    >
      {integration ? (
        <AgentLogo logo={integration.logo} size={size} />
      ) : (
        <span
          className="inline-block shrink-0 rounded-sm bg-rmb-fill"
          style={{ width: size, height: size }}
          aria-hidden
        />
      )}
      <span className="truncate">{label}</span>
    </span>
  );
}
