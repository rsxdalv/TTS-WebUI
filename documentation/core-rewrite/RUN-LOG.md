# Run log

The log for runs 1–3 is in [archive/RUN-LOG-runs-1-4.md](archive/RUN-LOG-runs-1-4.md).

## 2026-11-03 — Run 4 stopped; plan reworked into rings

- Run 4 (unattended director, wave R2.3c) hit a Copilot rate limit. It merged the C2.4, C3.2 and C3.1 fixes to
  `core/rewrite` at `41f6ecb`; vet and tests pass, but nothing has been clicked through or reviewed yet.
- It left an unfinished smoke rewrite and Playwright draft uncommitted on `core/r4-smoke` in the main checkout.
- Replaced the director and waves with rings and gates (D13–D15). Old plan, log, guide and agents are in `archive/`.
- Copilot logs for runs 1–3 and the start of Run 4: about 2,400 turns, 160 subagents (115 of them tiny),
  about 46% of tool calls were reads and searches, Haiku used about 10 times.
- Next: rings 1 and 2 with the `ring` agent, starting from "What is left" in `PLAN.md`.
