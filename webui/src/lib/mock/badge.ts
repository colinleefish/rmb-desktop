/**
 * Dev-only corner badge showing the current data mode and toggling it.
 * Amber "MOCK DATA" in mock mode, neutral "LIVE DATA" when proxied to the
 * real daemon. Click toggles localStorage `rmb.mock` and reloads; when the
 * mode is forced by VITE_MOCK (npm run dev:mock) the badge is inert.
 */
import { isMockEnvForced, isFullMock } from "../mockMode";

export function installMockBadge(): void {
  if (!import.meta.env.DEV || typeof document === "undefined") return;
  const forced = isMockEnvForced();
  const mock = isFullMock();

  const badge = document.createElement("button");
  badge.type = "button";
  badge.textContent = mock ? "MOCK DATA" : "LIVE DATA";
  badge.title = forced
    ? "Mock mode is forced by VITE_MOCK=true (npm run dev:mock). Restart without it to use the real daemon."
    : mock
      ? "Mock mode (localStorage rmb.mock). Click to reload against the real daemon proxy."
      : "Proxied to the real daemon. Click to switch to in-memory mock data.";
  Object.assign(badge.style, {
    position: "fixed",
    right: "12px",
    bottom: "12px",
    zIndex: "9999",
    padding: "4px 10px",
    borderRadius: "9999px",
    border: mock ? "1px solid rgba(217,119,6,0.5)" : "1px solid rgba(13,148,136,0.45)",
    background: mock ? "rgba(254,243,199,0.92)" : "rgba(204,251,241,0.92)",
    color: mock ? "#92400e" : "#115e59",
    font: "600 11px/1.4 ui-sans-serif, system-ui, sans-serif",
    letterSpacing: "0.06em",
    cursor: forced ? "default" : "pointer",
    boxShadow: "0 1px 4px rgba(0,0,0,0.15)",
    opacity: "0.9",
  } satisfies Partial<CSSStyleDeclaration>);
  badge.addEventListener("click", () => {
    if (forced) return;
    try {
      if (mock) localStorage.removeItem("rmb.mock");
      else localStorage.setItem("rmb.mock", "1");
    } catch {
      return;
    }
    window.location.reload();
  });
  document.body.appendChild(badge);
}
