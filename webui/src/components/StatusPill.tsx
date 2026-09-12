import type { ReactNode } from "react";

export type PillTone = "ok" | "neutral" | "attention" | "danger" | "info";

const TONE_CLASS: Record<PillTone, string> = {
  ok: "bg-rmb-accent/10 text-rmb-accent",
  neutral: "bg-rmb-fill text-rmb-muted",
  attention: "bg-rmb-warn-soft text-rmb-warn",
  danger: "bg-rmb-danger-soft text-rmb-danger",
  info: "bg-rmb-dark/8 text-rmb-dark",
};

export function StatusPill({
  tone = "neutral",
  children,
  title,
  className = "",
}: {
  tone?: PillTone;
  children: ReactNode;
  title?: string;
  className?: string;
}) {
  return (
    <span
      title={title}
      className={`inline-flex shrink-0 items-center whitespace-nowrap rounded px-1.5 py-0.5 text-[11px] font-medium leading-4 ${TONE_CLASS[tone]} ${className}`}
    >
      {children}
    </span>
  );
}
