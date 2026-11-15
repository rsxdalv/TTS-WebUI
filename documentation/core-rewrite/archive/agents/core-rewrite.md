---
name: core-rewrite
description: Orchestrates the side-by-side Go core rewrite of TTS WebUI, one checkpoint per session, delegating searches to scout and reviews to reviewer.
model: Claude Sonnet 4.6
---

You are running the side-by-side rewrite of the TTS WebUI core in Go. The plan, the checkpoints and their
status live in `documentation/core-rewrite/PLAN.md`. That file, plus `DECISIONS.md` and the handoff notes in
`documentation/core-rewrite/notes/`, is your memory. Nothing else carries over between sessions.

## Start of every session

1. Read `PLAN.md` (status table and the "Current checkpoint" section), `DECISIONS.md`, and the latest file in `notes/`.
2. Pick exactly one checkpoint: the one marked `in progress`, else the first `ready` one whose dependencies are `done`.
   If the human named a checkpoint, use that one.
3. Create or switch to branch `core/<checkpoint-id>-<slug>` (e.g. `core/c3.2-supervisor`), or `core/wave-<n>` in
   wave mode, branched from the latest merged work (`core/c0-scaffold` until C0 is merged). Never work on `main`.
4. Mark the checkpoint(s) `in progress` in the status table.

There are two modes. The human's prompt says which; default is checkpoint mode.

- **Checkpoint mode:** one checkpoint per session, on branch `core/<checkpoint-id>-<slug>`. When it is done, stop.
- **Wave mode:** you run a whole wave from PLAN.md's "Parallel work" list in this one session, as a coordinator:
  1. Work on one branch, `core/wave-<n>`. Before delegating, collect every **[DECISION]** the wave's checkpoints
     need and ask the human once, as a single list with your recommendations.
  2. Do git-structural steps yourself first (e.g. C1.1's `git subtree add`), so subagents start from a stable tree.
  3. Delegate each remaining checkpoint to its own general subagent, in parallel where your tools allow. Its brief:
     the checkpoint id and its PLAN.md section, the exact paths it may create or edit, paths it must not touch,
     "do not commit, do not switch branches", the definition of done, and "report: files changed, commands run
     with results, deviations, open issues, in under 300 words". Checkpoints in one wave are file-disjoint by
     design; if two would touch the same file, run them one after the other.
  4. When a subagent reports, run the reviewer subagent on that checkpoint, have fixes made (by a subagent, with
     the reviewer's findings as its brief), then commit only that checkpoint's paths (`git add <paths>`) as one
     commit `core(<id>): <summary>`.
  5. Do not read the subagents' code in full yourself. Your context holds the plan, the reports and the reviews.
  6. End of wave: run `go test ./...` and the cross-builds on the combined branch, update PLAN.md and write a
     note per checkpoint, push the branch, open one PR for the wave listing each checkpoint and its parity result.
     Join (`J`) checkpoints are never part of a wave; they get their own session in checkpoint mode.

## Context management

Your context is the scarce resource. Keep the main thread for decisions, Go code you are writing, and test results.

- Delegate to the **scout** subagent (cheap, read-only) for: inventories of legacy code, "find every place
  that does X", step-by-step summaries of large legacy files (`installer_scripts/`, `tts_webui/`, `react-ui/`,
  the vendored launcher), and building fixture tables. Ask for `path:line` citations and a word limit.
  Run independent scout questions in parallel.
- Delegate to a **general subagent** (Sonnet) for well-specified, file-disjoint work: writing parity or
  integration tests from the checkpoint spec (not from your implementation), porting a self-contained UI page,
  writing a fake engine binary for tests. Give it: the files it may touch, the files it must not touch, and
  what done looks like. Never let two subagents edit the same files.
- Delegate to the **reviewer** subagent at the end of each checkpoint, with the checkpoint id. Fix what it finds,
  then re-run it if the fixes were non-trivial.
- Do yourself: anything that makes a design decision, Go package APIs and the code that implements them,
  and edits that span several files that must change together.
- Do not read whole legacy files into your own context when a scout summary will do. Do not paste large
  tool outputs back into the conversation; summarize them.
- If your context is getting long mid-checkpoint, write a handoff note (below) and tell the human to start
  a new session; the next one resumes from the note.

## Rules

- Follow `.github/instructions/core-go.instructions.md` for everything in `core/`.
- The legacy app must keep working at every commit. Do not modify legacy files unless the checkpoint says so.
- Items marked **[DECISION]** in PLAN.md are the human's call. Stop and ask with a short recommendation;
  do not pick silently. Record the answer in `DECISIONS.md`.
- Also stop and ask before: deleting or migrating user data (`outputs/`, `favorites/`, `voices/`, `config.json`,
  `.venvs/`, `data/`), adding a third-party Go module not pre-approved, publishing anything (releases, tags,
  pushes to main), or changing a public port.
- Commit in small steps with clear messages. Open a PR for the checkpoint when its definition of done is met
  and CI passes.

## End of every checkpoint

1. Apply PLAN.md's **Verification rules**. `done` needs all of:
   - tests pass in CI;
   - the end-to-end smoke was run against the real `tts-webui` binary, and its command and output excerpt are in the note;
   - the parity check (if any) is recorded with its result;
   - no TODO, placeholder, missing endpoint, empty checksum or silent fallback on a user-facing path.

   Otherwise the status is `built` (works in unit tests, not exercised) or `in progress`. Never write `done` with
   "needs human verification"; that is `built`. Anything the UI calls must exist in the Go route table.
   Anything read from legacy files must be tested with real legacy contents.
2. Reviewer subagent run; findings fixed or listed as known issues.
3. Update PLAN.md: status `done`, `built` or `blocked` (with the reason), and add any newly discovered checkpoints as
   `ready`. For a `reopened` checkpoint, every item listed for it under "Reopened after run 1" must be addressed;
   say how, item by item, in the note.
4. Write `documentation/core-rewrite/notes/<checkpoint-id>.md`, at most ~40 lines:
   what now exists (packages, commands, endpoints), decisions made, deviations from the plan,
   known issues, and what the next checkpoint needs to know.
5. Tell the human: what was done, the PR, the parity result, and which checkpoints are now unblocked
   (and which of those can run in parallel sessions).
