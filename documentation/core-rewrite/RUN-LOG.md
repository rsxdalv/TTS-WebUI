# Run log

The log for runs 1–3 is in [archive/RUN-LOG-runs-1-4.md](archive/RUN-LOG-runs-1-4.md).

## 2026-11-10 — Run 5 continued: demo gate fixes (c511f4c)

- Bug 1: `.gpu` written with `\n`; `initializeApp.js` strict-equals failed. Fixed: write without newline.
- Bug 2: `MAMBA_ROOT_PREFIX` never set; micromamba fell back to `~\micromamba`. Fixed: `mambaEnv()` helper in steps.go used for handleInstall, handleUpdate, supervisor baseEnv.
- Bug 3: Origin check on all paths; Chrome's `crossorigin` attribute triggered 403 for CSS/JS. Fixed: origin check now only on `/api/*`; also pinned CSS/JS MIME types (Windows registry guard).
- vet + tests + smoke: all pass. Pushed c511f4c.
- Next: re-run demo gate (rings 1 and 2) from scratch install.



- Adopted uncommitted smoke work; m1.ps1 passes (PASS), 10 Playwright tests pass.
- Added Diagnostic button (POST /api/app/diagnostic + stream; handlers.go + LaunchStep.tsx + rebuilt bundle).
- Fixed: readGPULabel() now guards app/start in conda mode (was uv-only); install now 400s without mamba/gpu.
- m1.ps1: added restart + diagnostic stream assertions (two SKIP gaps from reviewer).
- 5/5 Run-3 review items pass. Both smoke scripts PASS. vet + all tests green. Pushed `4a4a362`.
- Next: demo gates (human) — ring 1 then ring 2 on a real install, no Neutralino, no spaces in path.
- Stats: 254 turns, 344 tool calls, 4 subagents.

## 2026-11-03 — Run 4 stopped; plan reworked into rings

- Run 4 (unattended director, wave R2.3c) hit a Copilot rate limit. It merged the C2.4, C3.2 and C3.1 fixes to
  `core/rewrite` at `41f6ecb`; vet and tests pass, but nothing has been clicked through or reviewed yet.
- It left an unfinished smoke rewrite and Playwright draft uncommitted on `core/r4-smoke` in the main checkout.
- Replaced the director and waves with rings and gates (D13–D15). Old plan, log, guide and agents are in `archive/`.
- Copilot logs for runs 1–3 and the start of Run 4: about 2,400 turns, 160 subagents (115 of them tiny),
  about 46% of tool calls were reads and searches, Haiku used about 10 times.
- Next: rings 1 and 2 with the `ring` agent, starting from "What is left" in `PLAN.md`.
