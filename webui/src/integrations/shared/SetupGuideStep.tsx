import type { ReactNode } from "react";

/** One sequential section of a setup guide (hook → recall → verify). Quiet
 * tabular-num badge, no bespoke circle/connector shape — matches the
 * hairline-section rhythm used by Settings' grouped fields. */
export function SetupGuideStep({
  step,
  isLast = false,
  children,
}: {
  step: number;
  isLast?: boolean;
  children: ReactNode;
}) {
  return (
    <section className={isLast ? "" : "mb-6 border-b border-rmb-line pb-6"}>
      <span
        className="mb-3 inline-flex size-5 items-center justify-center rounded bg-rmb-fill text-[11px] font-semibold tabular-nums text-rmb-muted"
        aria-hidden
      >
        {step}
      </span>
      {children}
    </section>
  );
}
