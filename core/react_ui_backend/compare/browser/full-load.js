const { chromium } = require("playwright-core");
(async () => {
  const headless = process.argv[2] !== "headed";
  const b = await chromium.launch({ channel: "msedge", headless });
  const ctx = await b.newContext();
  for (const [url, needle] of [["http://127.0.0.1:34012/outputs/outputs", "FRESH-OUTPUT"], ["http://127.0.0.1:34012/text-to-speech/bark/voices", "zz.fresh"]]) {
    const p = await ctx.newPage();
    const reqs = [];
    p.on("request", r => { if (r.url().includes("go-data")) reqs.push(r.url()); });
    const errs = []; p.on("console", m => { if (m.type()==="error") errs.push(m.text().slice(0,120)); });
    await p.goto(url);
    await p.waitForTimeout(6000);
    const r = await p.evaluate((n) => ({ vis: document.visibilityState, cookie: document.cookie, fresh: new RegExp(n,"i").test(document.body.innerText), fibers: [...document.querySelectorAll("*")].some(e => Object.keys(e).some(k => k.startsWith("__react"))) }), needle);
    console.log(headless ? "headless" : "headed", url.split("34012")[1], JSON.stringify(r), "go-data requests:", reqs, "errors:", errs);
    await p.close();
  }
  await b.close();
})();
