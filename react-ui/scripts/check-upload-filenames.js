/**
 * Browser regression for uploaded filenames and the FFmpeg metadata page.
 * Start Vite, then run: node scripts/check-upload-filenames.js [base url]
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
    const names = process.argv[3] ? [process.argv[3]] : ["ordinary.wav", "take#2.wav", "take%2.wav", "take%20 final 你好.wav"];
    let current;
    await page.route("**/api/upload", (route) =>
      route.fulfill({ json: { name: current } })
    );
    await page.route("**/api/go-data/host-status", (route) =>
      route.fulfill({ json: { installed: false, installable: false } })
    );
    await page.route("**/api/go-data/ffmpeg-metadata", (route) =>
      route.fulfill({ json: route.request().postDataJSON() })
    );
    await page.route("**/file-input-cache/**", (route) => {
      return route.fulfill({ body: wav(), contentType: "audio/wav" });
    });
    await page.goto(base + "/outputs/ffmpeg-metadata-page", { waitUntil: "domcontentloaded" });
    for (const name of names) {
      current = name;
      console.log("Testing", name);
      const expected = "/file-input-cache/" + encodeURIComponent(name);
      const [metadata, playback] = await Promise.all([
        page.waitForRequest((request) =>
          request.url().endsWith("/api/go-data/ffmpeg-metadata"), { timeout: 10000 }),
        page.waitForRequest((request) =>
          new URL(request.url()).pathname.startsWith("/file-input-cache/"), { timeout: 10000 }),
        page.locator("input[type=file]").setInputFiles({
          name, mimeType: "audio/wav", buffer: wav(),
        }),
      ]);
      assert.equal(metadata.postDataJSON().name, name);
      assert.equal(new URL(playback.url()).pathname, expected, "wrong playback URL for " + name);
      await page.getByRole("button", { name: "Clear" }).click();
    }
    console.log("ok: upload, browser audio URL and metadata name round-trip all fixtures");
  } finally {
    await browser.close();
  }
}

function wav() {
  const bytes = Buffer.alloc(1644);
  bytes.write("RIFF", 0);
  bytes.writeUInt32LE(bytes.length - 8, 4);
  bytes.write("WAVEfmt ", 8);
  bytes.writeUInt32LE(16, 16);
  bytes.writeUInt16LE(1, 20);
  bytes.writeUInt16LE(1, 22);
  bytes.writeUInt32LE(8000, 24);
  bytes.writeUInt32LE(16000, 28);
  bytes.writeUInt16LE(2, 32);
  bytes.writeUInt16LE(16, 34);
  bytes.write("data", 36);
  bytes.writeUInt32LE(bytes.length - 44, 40);
  return bytes;
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
