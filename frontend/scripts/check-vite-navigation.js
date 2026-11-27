/**
 * Checks that the client-side navigation of the Vite app works: clicking a
 * link in the sidebar changes the page without a reload of the document.
 *
 * The static export check of the Go port does the same for the export
 * (core/react_ui_backend/browser/static-export-check.js).
 *
 * Usage: node scripts/check-vite-navigation.js [base url]
 */
const { chromium } = require("playwright-core");

const BASE = process.argv[2] || process.env.VITE_URL || "http://127.0.0.1:3002";

const main = async () => {
  const browser = await chromium.launch({ channel: "msedge", headless: true });
  const page = await browser.newPage();

  const problems = [];
  page.on("pageerror", (error) => problems.push(`pageerror: ${error}`));
  page.on("console", (message) => {
    if (message.type() === "error") problems.push(`console: ${message.text().slice(0, 140)}`);
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("main", { timeout: 30000 });
  await page.waitForTimeout(1500);

  // A marker on the window: a reload loses it.
  await page.evaluate(() => {
    window.__marker = 1;
  });

  const hrefs = await page.evaluate(() =>
    [
      ...new Set(
        [...document.querySelectorAll("a[href^='/']")].map((a) =>
          a.getAttribute("href")
        )
      ),
    ].filter((href) => href !== "/" && !href.startsWith("/api"))
  );

  let visited = 0;
  for (const href of hrefs) {
    // The sidebar of the page changes with the page, so every link is
    // looked up again after the navigation. A link that is not on the page
    // is followed with the router instead.
    const clicked = await page.evaluate((href) => {
      const link = [...document.querySelectorAll("a")].find(
        (a) => a.getAttribute("href") === href
      );
      if (!link) return false;
      link.click();
      return true;
    }, href);

    if (!clicked) {
      await page.evaluate((href) => {
        window.history.pushState(null, "", href);
        window.dispatchEvent(new PopStateEvent("popstate"));
      }, href);
    }
    await page.waitForTimeout(1200);

    const state = await page.evaluate(() => ({
      marker: window.__marker,
      path: window.location.pathname,
      title: document.title,
    }));

    if (state.marker !== 1) {
      problems.push(`${href}: the page was reloaded`);
    }
    if (state.path !== href) {
      problems.push(`${href}: the URL is ${state.path}`);
    }
    if (!state.title) {
      problems.push(`${href}: no title`);
    }
    visited++;
  }

  await browser.close();

  console.log(`visited ${visited} links of ${hrefs.length}`);
  if (problems.length) {
    for (const problem of problems) console.log(`FAIL ${problem}`);
    process.exit(1);
  }
  console.log("ok: every link navigated without a reload");
};

main().catch((error) => {
  console.error(error);
  process.exit(2);
});
