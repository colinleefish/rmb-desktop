import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The Tauri desktop build and the Go backend serve the SPA under `/ui/`, so the
// production build must keep that base. The Vite dev server (used by the v0
// preview) serves from the root `/` so the preview renders without a base-path
// redirect.
export default defineConfig(({ command }) => ({
  base: command === "build" ? "/ui/" : "/",
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:19019",
      "/healthz": "http://127.0.0.1:19019",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
}));
