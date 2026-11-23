import { readFile } from "fs/promises";
import path from "path";
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";

const src = path.resolve(__dirname, "src");

/**
 * Next imports an `.svg` file as an asset object (`{ src, width, height }`).
 * Vite imports it as the URL string, so the icon components would lose their
 * dimensions. This plugin gives the same object shape, with the width and
 * the height read from the SVG itself.
 */
function svgAsAsset(): Plugin {
  return {
    name: "tts-svg-as-asset",
    enforce: "pre",
    async load(id) {
      // A query means Vite asks for the file itself (for example the `?url`
      // import below). That is Vite's own job, so the plugin steps aside.
      if (id.includes("?")) return null;
      if (!id.endsWith(".svg")) return null;
      const svg = await readFile(id, "utf8");
      const width = /width="([\d.]+)"/.exec(svg)?.[1] ?? "24";
      const height = /height="([\d.]+)"/.exec(svg)?.[1] ?? "24";
      // The file is imported with the `?url` suffix, so Vite serves it as a
      // file and gives back its URL.
      return [
        `import __url from ${JSON.stringify(id + "?url")};`,
        `export default { src: __url, width: ${width}, height: ${height} };`,
      ].join("\n");
    },
  };
}

/**
 * Swaps a module for another one in the Vite build.
 *
 * The shared sources are written for Next, which inlines `process.env` and
 * reads the voice data from the file system. The browser needs the
 * environment variables from `import.meta.env` and the voice data from the
 * API, so those modules have a browser version.
 *
 * The pattern matches the whole import specifier, so a relative import
 * (`../../data/env`) and an absolute one (`@/data/env`) are both replaced by
 * the absolute path of the browser module.
 */
const swappedModules = {
  // The environment variables (see src/data/env.ts).
  "data/env": "data/env.vite.ts",
  // The filesystem-backed voice data (see src/data/getVoicesData.tsx).
  "data/getVoicesData": "data/getVoicesData.browser.tsx",
  // The database client needs Node's TCP stack.
  "lib/sql": "lib/sql.browser.tsx",
};

/** An absolute path with forward slashes, for the alias replacement. */
const absolute = (...parts: string[]) =>
  path.resolve(src, ...parts).split(path.sep).join("/");

export default defineConfig({
  plugins: [svgAsAsset(), react()],
  resolve: {
    alias: [
      // The Next APIs the shared sources still use. The shims keep the call
      // sites unchanged (see src/shims/).
      { find: /^next\/router$/, replacement: absolute("shims/next-router.tsx") },
      { find: /^next\/navigation$/, replacement: absolute("shims/next-navigation.tsx") },
      { find: /^next\/link$/, replacement: absolute("shims/next-link.tsx") },
      { find: /^next\/head$/, replacement: absolute("shims/next-head.tsx") },
      { find: /^next\/dynamic$/, replacement: absolute("shims/next-dynamic.tsx") },
      { find: /^next\/font\/google$/, replacement: absolute("shims/next-font-google.tsx") },
      // Next ships a browser build of `path`; Vite does not.
      { find: /^path$/, replacement: "path-browserify" },
      // The modules with a browser version.
      ...Object.entries(swappedModules).map(([from, to]) => ({
        find: new RegExp(`^.*/${from}$`),
        replacement: absolute(to),
      })),
      { find: /^@\//, replacement: absolute("").replace(/\/$/, "") + "/" },
    ],
  },
  // The environment variables the shared sources read through process.env.
  // Vite inlines them at build time, so the browser bundle has no `process`.
  define: {
    "process.env.TTS_STATIC_EXPORT": JSON.stringify(
      process.env.TTS_STATIC_EXPORT ?? ""
    ),
    "process.env.NEXT_PUBLIC_DB_API_URL": JSON.stringify(
      process.env.NEXT_PUBLIC_DB_API_URL ?? ""
    ),
    // Server-only values: the browser never reads them.
    "process.env.GRADIO_BACKEND": "undefined",
    "process.env.GRADIO_BACKEND_AUTOMATIC": "undefined",
    "process.env.GRADIO_AUTH": "undefined",
  },
  server: {
    port: 3002,
    strictPort: true,
    // Listen on the loopback address only, like the Go proxy and Next do.
    host: "127.0.0.1",
    proxy: {
      // The API routes belong to the server. The Next dev server (port 3001)
      // answers them. Set VITE_API_PROXY to another server to use that one
      // instead: the Go proxy in front of Next answers the /api/go-data/*
      // routes as well, so both apps show the same data.
      "/api": {
        target: process.env.VITE_API_PROXY || "http://127.0.0.1:3001",
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on("proxyReq", (proxyReq) => {
            // The Go proxy checks the Origin of a request against its own
            // port (CONTRACTS.md section 5). The browser sends the Origin of
            // the Vite page, so it is rewritten to the target's origin.
            const origin = proxyReq.getHeader("origin");
            if (origin) {
              const target = new URL(
                process.env.VITE_API_PROXY || "http://127.0.0.1:3001"
              );
              proxyReq.setHeader("origin", target.origin);
            }
          });
        },
      },
    },
  },  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
