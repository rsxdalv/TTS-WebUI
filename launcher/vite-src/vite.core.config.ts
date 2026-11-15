import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "path";
import { fileURLToPath } from "url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));

// Build config for the Go-embedded core entry.
// This entry calls only /api/steps/* and /api/app/* — no Neutralino, no NL_* globals.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "dist-core",
    emptyOutDir: true,
    rollupOptions: {
      input: {
        index: resolve(__dirname, "core.html"),
      },
    },
  },
});
