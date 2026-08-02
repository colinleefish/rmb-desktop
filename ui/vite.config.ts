import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  base: "/ui/",
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
});
