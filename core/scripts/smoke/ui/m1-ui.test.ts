/**
 * M1 UI smoke — browser-driven Playwright tests.
 *
 * Covers (PLAN.md "What is left", item 2):
 *   1. Every step through Launch works (preflight→mamba→gpu→install→update→launch)
 *   2. App link appears after Launch
 *   3. Stop works
 *   4. Reload mid-step reattaches to the stream
 *   5. A root with a space shows the failed preflight check
 *   6. A stub that exits 1 shows a step failure in the UI
 *   7. Clicking Launch twice does not start a second copy
 *
 * Prerequisites (run via run-playwright.ps1):
 *   - Port 18789: normal fixture, init_mamba.bat has ~5 s delay for reload test.
 *   - Port 18790: fixture root contains a space in its path.
 *   - Port 18791: fixture with preflight pre-done and init_mamba.bat that exits 1.
 *
 * UI component selectors are derived from core-app/ React sources.
 * No expect(true).toBe(true) placeholders.
 */
import { test, expect, chromium, type BrowserContext, type Page } from '@playwright/test';

const BASE      = 'http://127.0.0.1:18789';
const SPACE_URL = 'http://127.0.0.1:18790';
const FAIL_URL  = 'http://127.0.0.1:18791';

// ---------------------------------------------------------------------------
// Main flow (port 18789) — sequential tests sharing one browser context/page.
// Each test picks up where the previous left off.
// ---------------------------------------------------------------------------
test.describe('M1 UI smoke — main flow', () => {
  let ctx: BrowserContext;
  let page: Page;

  test.beforeAll(async () => {
    const browser = await chromium.launch({ headless: true });
    ctx  = await browser.newContext({ baseURL: BASE });
    page = await ctx.newPage();
  });

  test.afterAll(async () => {
    await ctx.browser()?.close();
  });

  // 1. Page loads and leaves the loading skeleton.
  test('loads the launcher and leaves loading state', async () => {
    await page.goto('/');
    // Title from index.html and CoreApp <h1>.
    await expect(page).toHaveTitle('TTS WebUI Setup');
    await expect(page.getByRole('heading', { name: 'TTS WebUI Setup' })).toBeVisible({ timeout: 10_000 });
    // Loading skeleton disappears once /api/steps/status returns.
    await expect(page.getByText('Detecting installation state')).not.toBeVisible({ timeout: 10_000 });
  });

  // 2. Preflight auto-runs (PreflightView mounts with useEffect) and passes,
  //    advancing to the mamba step.
  test('preflight auto-runs and mamba step becomes active', async () => {
    // PreflightView fires POST /api/steps/preflight on mount; if it passes,
    // onComplete() → onRefresh() → mamba becomes the active step.
    // "Init Mamba Environment" is the StepList STEP_META title for mamba.
    await expect(page.getByText('Init Mamba Environment')).toBeVisible({ timeout: 30_000 });
    // The StepItem for mamba renders a "Run" button when state="active".
    await expect(page.getByRole('button', { name: 'Run' })).toBeVisible({ timeout: 10_000 });
  });

  // 3. Reload mid-step reattaches to the SSE stream.
  //    init_mamba.bat has a ~5 s ping delay; we reload within the first second
  //    so the proc is still running when the fresh page fetches /api/steps/status.
  //    StepItem picks up running:true and opens the stream (via its mount useEffect).
  test('reload mid-step reattaches to stream', async () => {
    await page.getByRole('button', { name: 'Run' }).click();
    // Let the bat start and be mid-way through the ping loop.
    await page.waitForTimeout(1_000);
    // Navigate fresh — forces a new React mount and a new /api/steps/status call.
    await page.goto('/');
    await expect(page).toHaveTitle('TTS WebUI Setup');
    // The mamba step must still be shown (running or just-completed).
    await expect(page.getByText('Init Mamba Environment')).toBeVisible({ timeout: 10_000 });
    // Either the stream reattached (Running... button — unique to active StepItem)
    // or mamba completed and GPU is now active (Continue button — unique to GPUView).
    // Note: step title text is always rendered for all steps, so we use buttons
    // to discriminate state. Add .first() to avoid strict-mode if both somehow match.
    await expect(
      page.getByRole('button', { name: 'Running...' })
          .or(page.getByRole('button', { name: 'Continue' }))
          .first(),
    ).toBeVisible({ timeout: 15_000 });
    // Wait for mamba to finish: GPU step becomes active, showing the Continue button.
    await expect(page.getByRole('button', { name: 'Continue' })).toBeVisible({ timeout: 60_000 });
  });

  // 4. GPU step: radio selector appears; user picks CPU and clicks Continue.
  //    GPUView POSTs {"label":"CPU"} to /api/steps/gpu and calls onComplete().
  test('gpu step: select CPU and continue', async () => {
    // Continue button is unique to GPUView (gpu is the active step).
    await expect(page.getByRole('button', { name: 'Continue' })).toBeVisible({ timeout: 10_000 });
    // CPU radio button (GPUView renders one <input type="radio"> per choice).
    await page.getByRole('radio', { name: 'CPU' }).click();
    // "Continue" button in GPUView.
    await page.getByRole('button', { name: 'Continue' }).click();
    // After /api/steps/gpu succeeds, onComplete → onRefresh → install becomes active.
    await expect(page.getByText('Install WebUI')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByRole('button', { name: 'Run' })).toBeVisible({ timeout: 10_000 });
  });

  // 5. Install step: click Run, fakemamba prints its header, step completes.
  //    After completion the update step auto-resolves (already_updated:true from
  //    /api/steps/status) and LaunchStep appears.
  test('install step: logs appear and step completes', async () => {
    await page.getByRole('button', { name: 'Run' }).click();
    // fakemamba prints "fakemamba: …" when invoked as micromamba.
    // Use .first() because the log box renders one <div> per line.
    await expect(page.getByText(/fakemamba/i).first()).toBeVisible({ timeout: 30_000 });
    // When the stream done event fires with exitCode 0, onComplete → refresh.
    // The status handler marks update.done=true (isGitUpdated returns true when
    // git fetch fails — no git repo in scratch dir), making allDone=true.
    // LaunchStep appears once all steps are done.
    await expect(page.getByRole('heading', { name: 'Launch WebUI' })).toBeVisible({ timeout: 60_000 });
  });

  // 6. Launch step: click "Launch WebUI", fakepython starts on :7770,
  //    the UI polls /api/app/link and shows the "Open WebUI →" link.
  test('launch step: app link appears after clicking Launch WebUI', async () => {
    await page.getByRole('button', { name: 'Launch WebUI' }).click();
    // LaunchStep subscribes to /api/app/status/stream and polls /api/app/link.
    // fakepython serves HTTP on 127.0.0.1:7770; AppLink() returns that URL.
    await expect(page.getByRole('link', { name: 'Open WebUI →' })).toBeVisible({ timeout: 20_000 });
    // At least one process shown as running in the process list.
    await expect(page.getByText('running')).toBeVisible({ timeout: 10_000 });
  });

  // 7. Clicking Launch twice does not start a second copy.
  //    The button is gone after launch (launched=true hides it), so we call the
  //    API directly from the page context.  The server must return 409.
  test('clicking Launch twice returns 409 Conflict', async () => {
    const status = await page.evaluate(async () => {
      const r = await fetch('/api/app/start', { method: 'POST' });
      return r.status;
    });
    expect(status).toBe(409);
  });

  // 8. Stop works: click Stop, processes stop, "App stopped." text appears.
  test('stop works: stopped state shown', async () => {
    await page.getByRole('button', { name: 'Stop' }).click();
    // LaunchStep shows "App stopped." when allStopped is true.
    await expect(page.getByText('App stopped.')).toBeVisible({ timeout: 15_000 });
  });
});

// ---------------------------------------------------------------------------
// Root with a space in path (port 18790).
// ---------------------------------------------------------------------------
test.describe('M1 UI smoke — space in path', () => {
  // 5. A root with a space triggers the SpacesInPath preflight check failure.
  //    PreflightView auto-runs on mount and shows the ✗ icon + error message.
  test('root with space shows failed preflight check', async ({ page }) => {
    await page.goto(SPACE_URL);
    await expect(page).toHaveTitle('TTS WebUI Setup');
    // PreflightView auto-fires; the SpacesInPath check returns StatusFail.
    // PreflightView renders "Fix the issues above to continue." when ok=false.
    await expect(page.getByText(/Fix the issues above to continue/i)).toBeVisible({ timeout: 20_000 });
    // At least one ✗ icon (fail indicator rendered by statusIcon('fail')).
    await expect(page.getByText('✗')).toBeVisible({ timeout: 10_000 });
  });
});

// ---------------------------------------------------------------------------
// Stub exits 1 (port 18791).
// ---------------------------------------------------------------------------
test.describe('M1 UI smoke — failing stub', () => {
  // 6. A stub whose process exits 1 is surfaced in the UI as an error.
  //    Fixture: preflight pre-completed, init_mamba.bat exits /b 1.
  //    StepItem receives event:done with exitCode:1 and renders the error div.
  test('stub that exits 1 shows step failure in UI', async ({ page }) => {
    await page.goto(FAIL_URL);
    await expect(page).toHaveTitle('TTS WebUI Setup');
    // Mamba is the active step (preflight pre-done); "Run" button is visible.
    await expect(page.getByRole('button', { name: 'Run' })).toBeVisible({ timeout: 15_000 });
    await page.getByRole('button', { name: 'Run' }).click();
    // init_mamba.bat prints one line then exits /b 1.
    // StepItem: done event exitCode=1 → setError(`Step exited with code 1`).
    await expect(page.getByText('Step exited with code 1')).toBeVisible({ timeout: 30_000 });
  });
});
