import { readFile } from "fs/promises";
import path from "path";
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";

const src = path.resolve(__dirname, "src");

/**
 * The icon components read `{ src, width, height }` from an imported `.svg`
 * file. Vite imports it as the URL string, so the icon components would lose
 * their dimensions. This plugin gives the object, with the width and the
 * height read from the SVG itself.
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

/** An absolute path with forward slashes, for the alias replacement. */
const absolute = (...parts: string[]) =>
  path.resolve(src, ...parts).split(path.sep).join("/");

export default defineConfig({
  plugins: [svgAsAsset(), react()],
  resolve: {
    alias: [
      // The shared sources import `path`; Vite has no browser build of it.
      { find: /^path$/, replacement: "path-browserify" },
      { find: /^@\//, replacement: absolute("").replace(/\/$/, "") + "/" },
    ],
  },
  server: {
    port: 3002,
    strictPort: true,
    // Listen on the loopback address only, like the Go server does.
    host: "127.0.0.1",
    proxy: {
      // The API routes belong to the Go server. Start the app with the
      // launcher (it listens on port 3000), or set VITE_API_PROXY to another
      // address.
      "/api": {
        target: process.env.VITE_API_PROXY || "http://127.0.0.1:3000",
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on("proxyReq", (proxyReq) => {
            // The Go server checks the Origin of a request against its own
            // port (CONTRACTS.md section 5). The browser sends the Origin of
            // the Vite page, so it is rewritten to the target's origin.
            const origin = proxyReq.getHeader("origin");
            if (origin) {
              const target = new URL(
                process.env.VITE_API_PROXY || "http://127.0.0.1:3000"
              );
              proxyReq.setHeader("origin", target.origin);
            }
          });
        },
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
