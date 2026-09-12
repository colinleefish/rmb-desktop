import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The Tauri desktop build and the Go backend serve the SPA under `/ui/`, so the
// production build must keep that base. The Vite dev server (used by the v0
// preview) serves from the root `/` so the preview renders without a base-path
// redirect. `npm run dev` proxies to the real daemon (default 127.0.0.1:19019,
// the rmbd DefaultAddr — override with RMB_API_TARGET); `npm run dev:mock`
// needs no daemon at all (VITE_MOCK=true serves /api from lib/mock in-page).
const apiTarget = process.env.RMB_API_TARGET ?? "http://127.0.0.1:19019";

export default defineConfig(({ command }) => ({
  base: command === "build" ? "/ui/" : "/",
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      "/api": apiTarget,
      "/healthz": apiTarget,
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
}));
