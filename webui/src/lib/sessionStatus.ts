export type DistillStatus = "distilled" | "distilling" | "pending" | "failed";

/** Collapse the three tier statuses into the one word a user cares about. */
export function distillStatus(t1?: string, t2?: string, t3?: string): DistillStatus {
  const s = [t1, t2, t3].map((x) => (x ?? "").toLowerCase());
  if (s.some((x) => x === "failed")) return "failed";
  if (s.some((x) => x === "running")) return "distilling";
  if (s.some((x) => x === "pending" || x === "waiting" || x === "")) return "pending";
  return "distilled";
}

/** Raw tier statuses for tooltips: "T1 idle · T2 running · T3 waiting". */
export function tierStatusTitle(t1?: string, t2?: string, t3?: string): string {
  return [["T1", t1], ["T2", t2], ["T3", t3]]
    .map(([k, v]) => `${k} ${v || "—"}`)
    .join(" · ");
}
