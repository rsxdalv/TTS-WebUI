// Loads every page of a static export in a real browser (Edge), through the Go
// proxy in front of staticsrv.js, and reports hydration, console errors, failed
// requests and client-side navigation. `node static-export-check.js <base url>
// <out dir> [<extension name>] [-gradio]`. Exit code 1 if anything is wrong.
const { chromium } = require("playwright-core");
const fs = require("fs");
const path = require("path");

const gradio = process.argv.includes("-gradio");
const [base, out, extName] = [process.argv[2], process.argv[3], process.argv[4]];

function pages(dir, prefix = "") {
  let r = [];
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (e.isDirectory()) { if (!e.name.startsWith("_")) r = r.concat(pages(path.join(dir, e.name), prefix + "/" + e.name)); }
    else if (e.name.endsWith(".html") && !["404.html", "_not-found.html"].includes(e.name)) {
      const n = e.name.slice(0, -5);
      r.push(n === "index" ? prefix || "/" : prefix + "/" + n);
    }
  }
  return r;
}

const hydrated = () => [...document.querySelectorAll("*")].some((e) => Object.keys(e).some((k) => k.startsWith("__react")));

(async () => {
  const urls = [...new Set(pages(out))].sort();
  if (extName) urls.push("/extensions/" + extName);
  const b = await chromium.launch({ channel: "msedge", headless: true });
  const ctx = await b.newContext();
  let bad = 0;
  for (const u of urls) {
    const p = await ctx.newPage();
    const problems = [];
    p.on("console", (m) => { if (m.type() === "error" && (gradio || !/not valid JSON/.test(m.text())) && !/\((Not Found|Internal Server Error)\)/.test(m.text())) problems.push("console: " + m.text().slice(0, 140)); });
    p.on("pageerror", (e) => { if (gradio || !/not valid JSON/.test(String(e))) problems.push("pageerror: " + String(e).slice(0, 140)); });
    p.on("requestfailed", (r) => { if (!r.url().startsWith("blob:")) problems.push("requestfailed: " + r.url()); });
    // Preview images of generations and voices only exist for real data, and
    // /api/gradio/* needs the Python backend: pass -gradio to count those.
    p.on("response", (r) => { if (r.status() >= 400 && !r.url().includes("/api/webui-generations/") && (gradio || !r.url().includes("/api/gradio/"))) problems.push(`${r.status()} ${r.url()}`); });
    await p.goto(base + u);
    let ok = false;
    for (let i = 0; i < 40 && !ok; i++) { ok = await p.evaluate(hydrated); if (!ok) await p.waitForTimeout(250); }
    await p.waitForTimeout(500);
    if (!ok) problems.push("not hydrated");
    const title = await p.evaluate(() => document.title);
    console.log((problems.length ? "FAIL " : "ok   ") + u.padEnd(48) + " " + title + (problems.length ? "\n       " + problems.join("\n       ") : ""));
    if (problems.length) bad++;
    await p.close();
  }

  // Client-side navigation: follow links in the sidebar without a page load.
  const p = await ctx.newPage();
  await p.goto(base + "/");
  for (let i = 0; i < 40 && !(await p.evaluate(hydrated)); i++) await p.waitForTimeout(250);
  await p.evaluate(() => { window.__marker = 1; });
  const hrefs = await p.evaluate(() => [...new Set([...document.querySelectorAll("a[href^='/']")].map((a) => a.getAttribute("href")))].filter((h) => h !== "/" && !h.startsWith("/api") && !h.startsWith("/_next")));
  let nav = 0;
  for (const h of hrefs.slice(0, 12)) {
    await p.evaluate((h) => { const a = [...document.querySelectorAll("a")].find((x) => x.getAttribute("href") === h); a && a.click(); }, h);
    await p.waitForTimeout(700);
    const r = await p.evaluate(() => ({ path: location.pathname, marker: window.__marker }));
    const good = r.path === h && r.marker === 1;
    if (!good) { bad++; console.log("FAIL nav " + h + " -> " + JSON.stringify(r)); } else nav++;
    await p.goBack(); await p.waitForTimeout(400);
  }
  console.log(`client navigation: ${nav} of ${Math.min(12, hrefs.length)} links stayed in the page (links on /: ${hrefs.length})`);
  await b.close();
  console.log(bad ? `${bad} problem(s)` : "all good");
  process.exit(bad ? 1 : 0);
})();
