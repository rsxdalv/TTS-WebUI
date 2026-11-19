const { chromium } = require("playwright-core");
(async () => {
  const b = await chromium.launch({ channel: "msedge", headless: true });
  for (const mode of ["normal", "rAF never fires", "rAF fires late (8s)"]) {
    const ctx = await b.newContext();
    const p = await ctx.newPage();
    if (mode === "rAF never fires") await p.addInitScript(() => { window.requestAnimationFrame = () => 0; });
    if (mode.startsWith("rAF fires late")) await p.addInitScript(() => { const raf = window.requestAnimationFrame.bind(window); window.requestAnimationFrame = (cb) => setTimeout(() => raf(cb), 8000); });
    const reqs = []; p.on("request", r => { if (r.url().includes("go-data")) reqs.push(1); });
    await p.goto("http://127.0.0.1:34012/outputs/outputs");
    await p.waitForTimeout(4000);
    const at4 = await p.evaluate(() => ({ hydrated: [...document.querySelectorAll("*")].some(e => Object.keys(e).some(k => k.startsWith("__react"))), fresh: document.body.innerText.includes("FRESH-OUTPUT") }));
    await p.waitForTimeout(8000);
    const at12 = await p.evaluate(() => ({ hydrated: [...document.querySelectorAll("*")].some(e => Object.keys(e).some(k => k.startsWith("__react"))), fresh: document.body.innerText.includes("FRESH-OUTPUT") }));
    console.log(mode.padEnd(22), "t=4s", JSON.stringify(at4), "t=12s", JSON.stringify(at12), "go-data requests:", reqs.length);
    await ctx.close();
  }
  await b.close();
})();
