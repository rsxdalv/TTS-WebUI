import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import tailwindcss from "@tailwindcss/vite";
import { nodePolyfills } from "vite-plugin-node-polyfills";
import { resolve } from "path";
import { fileURLToPath } from "url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));

// Swap the neutralino adapter for the core adapter when VITE_PLATFORM=core.
const isPlatformCore = process.env.VITE_PLATFORM === "core";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), nodePolyfills({ include: ["path"] })],
  resolve: {
    alias: isPlatformCore
      ? [
          {
            find: /.*\/platform\/neutralino/,
            replacement: resolve(__dirname, "src/platform/core.ts"),
          },
        ]
      : [],
  },
});
