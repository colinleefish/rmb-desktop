/** Underline tab styling shared by PageTabs and the button-driven tab rows. */
export function tabClass(active: boolean): string {
  return [
    "-mb-px inline-flex h-9 items-center gap-1.5 border-b-2 px-2.5 text-sm transition-colors",
    active
      ? "border-rmb-accent font-medium text-rmb-dark"
      : "border-transparent text-rmb-muted hover:text-rmb-dark",
  ].join(" ");
}
