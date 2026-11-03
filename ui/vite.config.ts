import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vite.dev/config/
export default defineConfig({
  base: "/",
  plugins: [react()],
  build: {
    // Output to the Go embed directory so `go build` picks it up.
    outDir: "../core/internal/webui/assets",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      // Forward /api requests to the Go core server during development.
      "/api": {
        target: "http://127.0.0.1:7700",
        changeOrigin: true,
      },
    },
  },
});
