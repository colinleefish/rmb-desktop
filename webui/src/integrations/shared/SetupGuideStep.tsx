import type { ReactNode } from "react";

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
    <section className={`relative ${isLast ? "" : "pb-10"}`}>
      <span
        className="absolute -left-10 top-0 z-[1] flex size-7 items-center justify-center rounded-full border border-rmb-gray/25 bg-white text-xs font-semibold text-rmb-dark shadow-sm"
        aria-hidden
      >
        {step}
      </span>
      {!isLast && (
        <div
          className="absolute -left-[27px] top-7 bottom-0 w-px bg-rmb-gray/30"
          aria-hidden
        />
      )}
      {children}
    </section>
  );
}
