import type { AgentRegistryEntry } from "../../lib/agentRegistry";

export function AgentLogo({
  agent,
  inactive = false,
  size = 16,
}: {
  agent: Pick<AgentRegistryEntry, "logo" | "label">;
  inactive?: boolean;
  size?: number;
}) {
  const box = { width: size, height: size, minWidth: size, minHeight: size };

  return (
    <span
      className={[
        "inline-flex shrink-0 items-center justify-center overflow-hidden",
        inactive ? "opacity-35 grayscale" : "",
      ].join(" ")}
      style={box}
      aria-hidden
    >
      <img
        src={`${import.meta.env.BASE_URL}agents/${agent.logo}`}
        alt=""
        width={size}
        height={size}
        className="block object-contain"
        style={box}
        draggable={false}
      />
    </span>
  );
}
