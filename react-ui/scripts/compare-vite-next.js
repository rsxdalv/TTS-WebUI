/**
 * Compares the Vite dev server with the Next dev server, route by route.
 *
 * Both servers run side by side (Next on 3001, Vite on 3002). For every
 * route the script loads the page in a browser, reads the visible text and
 * the console messages, and reports the differences.
 *
 * Usage: node scripts/compare-vite-next.js [--routes=a,b]
 */
const { chromium } = require("playwright-core");
const fs = require("fs");
const path = require("path");

const NEXT = process.env.NEXT_URL || "http://127.0.0.1:3001";
const VITE = process.env.VITE_URL || "http://127.0.0.1:3002";

const ROUTES = [
  "/",
  "/pipeline",
  "/text-to-speech",
  "/text-to-speech/bark",
  "/text-to-speech/bark/generation",
  "/text-to-speech/bark/bark_voice_generation",
  "/text-to-speech/bark/voices",
  "/text-to-speech/bark/bark_settings",
  "/text-to-speech/bark/vocos_npz",
  "/text-to-speech/chatterbox",
  "/text-to-speech/kokoro",
  "/text-to-speech/maha-tts",
  "/text-to-speech/minimax-tts",
  "/text-to-speech/mms",
  "/text-to-speech/tortoise",
  "/text-to-speech/vallex",
  "/audio-music-generation",
  "/audio-music-generation/ace-step",
  "/audio-music-generation/magnet",
  "/audio-music-generation/musicgen",
  "/audio-music-generation/stable-audio",
  "/audio-conversion",
  "/audio-conversion/demucs",
  "/audio-conversion/rvc",
  "/audio-conversion/vocos_wav",
  "/outputs",
  "/outputs/outputs",
  "/outputs/favorites",
  "/outputs/ffmpeg-metadata-page",
  "/outputs/generations",
  "/outputs/voice-drafts",
  "/tools",
  "/tools/gpu_info",
  "/tools/api_config",
  "/extensions/huggingface_cache_manager",
  "/extensions/extension_xtts_simple",
  "/db",
  "/placeholder",
  "/test-gradio-proxy",
];

/**
 * The text of the page, without the parts that change between the two.
 *
 * The GPU widget shows live values (the temperature, the power draw), so
 * those are replaced by a placeholder: they differ between two loads of the
 * same page.
 */
const visibleText = (page) =>
  page.evaluate(() => {
    const main = document.querySelector("main") || document.body;
    const text = (main.innerText || "")
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .join("\n");
    // A number with a unit: the live values of the GPU widget.
    return text.replace(/\b\d+(?:\.\d+)?\s*(?:W|°C|C|%|MB|GB)(?![a-zA-Z])/g, "<live>");
  });

/** The errors the console shows. The HMR websocket of Next is not an error of the page. */
const consoleErrors = (messages) =>
  messages
    .filter(
      (message) =>
        message.type() === "error" &&
        !message.text().includes("_next/webpack-hmr") &&
        !message.text().includes("WebSocket connection")
    )
    .map((message) => message.text().slice(0, 200));

/**
 * The error kinds, without the parts that change (a value, a line number).
 * Two builds may report the same warning with a different text, so the
 * comparison is by kind.
 */
const errorKind = (text) => {
  if (text.includes("unique \"key\" prop")) return "react key warning";
  if (text.includes("cannot be a descendant of")) return "invalid DOM nesting";
  if (text.includes("Failed to fetch dynamically imported module"))
    return "module load failed";
  if (text.includes("status of 500")) return "500 response";
  if (text.includes("hydrat")) return "hydration warning";
  if (text.includes("validateDOMNesting")) return "invalid DOM nesting";
  return text.slice(0, 80);
};

const load = async (browser, base, route) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const messages = [];
  page.on("console", (message) => messages.push(message));
  page.on("pageerror", (error) =>
    messages.push({ type: () => "error", text: () => String(error) })
  );

  const response = await page.goto(base + route, {
    waitUntil: "domcontentloaded",
    timeout: 60000,
  });
  // The pages load their data after the first render. The dev servers keep
  // connections open, so there is no "network idle" to wait for.
  await page.waitForSelector("main, body > div", { timeout: 30000 });
  await page.waitForTimeout(2500);

  const result = {
    status: response ? response.status() : 0,
    title: await page.title(),
    text: await visibleText(page),
    errors: consoleErrors(messages),
  };
  await context.close();
  return result;
};

const main = async () => {
  const args = process.argv.slice(2);
  const routesArg = args.find((a) => a.startsWith("--routes="));
  const routes = routesArg
    ? routesArg.slice("--routes=".length).split(",")
    : ROUTES;

  const browser = await chromium.launch({ channel: "msedge", headless: true });
  let different = 0;

  for (const route of routes) {
    const next = await load(browser, NEXT, route);
    const vite = await load(browser, VITE, route);

    const problems = [];
    if (next.status !== vite.status) {
      problems.push(`status ${next.status} vs ${vite.status}`);
    }
    if (next.title !== vite.title) {
      problems.push(`title "${next.title}" vs "${vite.title}"`);
    }
    if (next.text !== vite.text) {
      problems.push("the visible text differs");
    }
    // Only the errors that Next does not have count: the app has warnings of
    // its own (the React key warning of the GPU widget, for example).
    const nextKinds = new Set(next.errors.map(errorKind));
    const newErrors = vite.errors.filter((error) => !nextKinds.has(errorKind(error)));
    if (newErrors.length) {
      problems.push(`new console errors: ${newErrors.join(" | ")}`);
    }

    if (problems.length) {
      different++;
      console.log(`DIFF ${route}`);
      for (const problem of problems) console.log(`     ${problem}`);
      if (next.text !== vite.text) {
        const nextLines = next.text.split("\n");
        const viteLines = vite.text.split("\n");
        for (let i = 0; i < Math.max(nextLines.length, viteLines.length); i++) {
          if (nextLines[i] !== viteLines[i]) {
            console.log(`     line ${i + 1}:`);
            console.log(`       next: ${JSON.stringify(nextLines[i])}`);
            console.log(`       vite: ${JSON.stringify(viteLines[i])}`);
            break;
          }
        }
      }
    } else {
      console.log(`ok   ${route}`);
    }
  }

  await browser.close();
  console.log(`\n${routes.length - different}/${routes.length} routes are the same.`);
  process.exit(different === 0 ? 0 : 1);
};

main().catch((error) => {
  console.error(error);
  process.exit(2);
});
