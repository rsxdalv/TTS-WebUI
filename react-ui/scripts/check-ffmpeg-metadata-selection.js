/**
 * Browser regression for delayed metadata of a no-longer-selected upload.
 * Start Vite, then run: node scripts/check-ffmpeg-metadata-selection.js [base url]
 * CHROMIUM_EXECUTABLE may point to an installed Chromium.
 */
const assert = require("node:assert/strict");
const { chromium } = require("playwright-core");

const base = process.argv[2] || "http://127.0.0.1:3002";
const launch = process.env.CHROMIUM_EXECUTABLE
  ? { executablePath: process.env.CHROMIUM_EXECUTABLE }
  : { channel: "msedge" };

async function main() {
  const browser = await chromium.launch(launch);
  try {
    const page = await browser.newPage();
    page.on("pageerror", (error) => console.error(error));
    page.setDefaultTimeout(10000);
    // Hold JSON results at the browser fetch boundary, including after abort.
    // This verifies that a completed but stale body cannot overwrite the UI.
    await page.addInitScript(() => {
      const fetchOriginal = window.fetch;
      window.metadataReplies = {};
      window.fetch = (input, init) => {
        if (input !== "/api/go-data/ffmpeg-metadata") return fetchOriginal(input, init);
        const { name } = JSON.parse(init.body);
        return Promise.resolve({
          ok: name !== "late-error.wav",
          status: name === "late-error.wav" ? 503 : 200,
          json: () => new Promise((resolve) => {
            window.metadataReplies[name] = resolve;
          }),
        });
      };
    });
    let current;
    await page.route("**/api/upload", (route) =>
      route.fulfill({ json: { name: current } })
    );
    await page.route("**/api/go-data/host-status", (route) =>
      route.fulfill({ json: { installed: false, installable: false } })
    );
    await page.route("**/file-input-cache/**", (route) => route.fulfill({ status: 404 }));
    await page.goto(base + "/outputs/ffmpeg-metadata-page", { waitUntil: "domcontentloaded" });

    async function select(name) {
      current = name;
      await page.locator("input[type=file]").setInputFiles({
        name, mimeType: "audio/wav", buffer: Buffer.from("RIFF"),
      });
      await page.waitForFunction((name) => Boolean(window.metadataReplies[name]), name);
    }
    async function resolve(name) {
      await page.evaluate(async (name) => {
        window.metadataReplies[name]({ marker: name, error: name });
        // Let the awaited continuation and React commit both run.
        await new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done)));
      }, name);
    }
    const clear = () => page.getByRole("button", { name: "Clear" }).click();
    if (process.argv[3] !== "switch") {
      await select("late-clear.wav");
      await clear();
      await resolve("late-clear.wav");
      assert.equal(await page.getByText("No metadata loaded").count(), 1);

    }
    if (process.argv[3] !== "clear") {
      await select("late-error.wav");
      await clear();
      await select("current.wav");
      await resolve("current.wav");
      await page.getByText(/current\.wav/).waitFor();
      await resolve("late-error.wav");
      assert.equal(await page.getByText(/current\.wav/).count(), 1);
      assert.equal(await page.getByText(/ffprobe is not installed/).count(), 0);
    }
    console.log("ok: delayed metadata cannot overwrite a clear or the newer selection");
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
