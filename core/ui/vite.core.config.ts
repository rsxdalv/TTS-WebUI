import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react-swc";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "path";
import { fileURLToPath } from "url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));

// Dev only (`npm run dev`): the page talks to a running launcher.
// LAUNCHER_URL picks it (default http://127.0.0.1:7700, the launcher's default
// port). The launcher accepts only its own Host/Origin and a per-run token, so
// the proxy rewrites Host and Origin and adds the token it reads from the
// launcher's index page. A launcher restart gives a new token; it is read again
// on the next request that is refused and on every page load.
const launcherURL = (process.env.LAUNCHER_URL || "http://127.0.0.1:7700").replace(/\/$/, "");
const launcherOrigin = new URL(launcherURL).origin;
const launcherPort = new URL(launcherURL).port || "80";

let launcherToken = "";

async function readLauncherToken(): Promise<string> {
  try {
    const res = await fetch(launcherURL + "/", { cache: "no-store" });
    const m = /<meta name="tts-webui-token" content="([^"]+)"/.exec(await res.text());
    launcherToken = m ? m[1] : "";
  } catch {
    launcherToken = "";
  }
  return launcherToken;
}

function launcherDev(): Plugin {
  return {
    name: "launcher-dev",
    apply: "serve",
    configureServer(server) {
      // Before the proxy: make sure there is a token to send. The page is
      // core.html, so open it at "/" too.
      server.middlewares.use(async (req, _res, next) => {
        if (req.url === "/" || req.url?.startsWith("/?")) req.url = "/core.html" + req.url.slice(1);
        if (req.url?.startsWith("/api/") && !launcherToken) await readLauncherToken();
        next();
      });
    },
    async transformIndexHtml() {
      const token = await readLauncherToken();
      if (!token) {
        console.warn(`[launcher-dev] no token from ${launcherURL}; is the launcher running? (LAUNCHER_URL)`);
        return [];
      }
      return [{ tag: "meta", attrs: { name: "tts-webui-token", content: token }, injectTo: "head-prepend" }];
    },
  };
}

// Build config for the Go-embedded core entry.
// This entry calls only /api/steps/* and /api/app/* — no Neutralino, no NL_* globals.
export default defineConfig({
  plugins: [react(), tailwindcss(), launcherDev()],
  server: {
    host: "127.0.0.1",
    proxy: {
      "/api": {
        target: launcherURL,
        changeOrigin: true, // Host becomes the launcher's
        configure: (proxy) => {
          proxy.on("proxyReq", (proxyReq) => {
            if (proxyReq.getHeader("origin")) proxyReq.setHeader("origin", launcherOrigin);
            if (launcherToken) {
              proxyReq.setHeader("X-TTS-Token", launcherToken);
              // EventSource cannot set headers; the launcher also takes the cookie.
              proxyReq.setHeader("Cookie", `tts-token-${launcherPort}=${launcherToken}`);
            }
          });
          proxy.on("proxyRes", (proxyRes) => {
            if (proxyRes.statusCode === 403) launcherToken = ""; // read again next time
          });
        },
      },
    },
  },
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
