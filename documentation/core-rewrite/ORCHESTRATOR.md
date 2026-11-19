# Orchestrator brief: Go core port (installer scripts + react-ui backend)

Start state: branch `core/go-port` (cut from `core/rings-1-2`, which holds the launcher, supervisor and setup page).
Read `PLAN.md`, `DECISIONS.md` and `core/README.md` first.

> **Corrected on 2026-11-19 (D16).** The shared brief in `tts-comms/BRIEF.md` is binding and wins wherever this file
> disagrees with it. `CONTRACTS.md` holds the technical contracts. Tasks run as detached sessions that sequence
> themselves through the comms files, not as waves started by an orchestrator.

## Division of work

Models: Haiku = exact spec, little judgement. Sonnet = a port with a clear original to match. Opus = a mistake here is costly (contracts, streaming/auth, install logic) and the reviews.

| ID | Task | Output path | Original size (lines) | Model | Depends on |
|---|---|---|---|---|---|
| C0 | Contracts doc: marker files, env vars, step-runner interface, route-claim table, auth/origin policy for port 3000, side-by-side test protocol | `documentation/core-rewrite/CONTRACTS.md` | small | Opus | none |
| H0 | Inventory: each `installer_scripts` file's inputs, outputs, callers; each Next route and `getStaticProps` page with its filesystem reads | `tts-comms/inventory.md` | read-only | Haiku | none |
| I1 | `init_mamba` (bat + sh) | `core/installer_scripts/init_mamba.go` | 164 + 210 | Sonnet | C0 |
| I2 | `root` (ps1 + sh) | `root.go` | 133 + 47 | Sonnet | C0 |
| I3 | `diagnostic` (bat, inner sh, wrapper) | `diagnostic.go` | 100 + 96 + 13 | Sonnet | C0 |
| I4 | `start_app`, `activate`, `open_terminal`, `run_extension_manager`, `init_app.bat/.sh` | one `.go` per script | 4 to 19 each | Haiku | C0 |
| I5 | `applyDatabaseConfig.js` | `apply_database_config.go` | 80 | Sonnet | C0 |
| I6 | `initializeApp.js` + `init_app.js` flow (torch/xformers pins, pip, extensions, DB config call, react-ui build) | `initialize_app.go`, `init_app_nodejs.go` | 360 + 127 | Opus | C0, I5, B1 |
| I7 | Per-step switch in the launcher (script by default, or Go), marker-file compatibility tests | `core/internal/launcher/*` | small | Sonnet | I1-I6, B1 |
| B1 | Go front proxy on port 3000, Next on an internal port, nothing claimed yet | `core/react_ui_backend/proxy.go`, the supervisor change, the proxy switch | medium | Opus | C0, BL |
| T1 | Side-by-side harness: replay a request against Next and Go, compare status, key headers, body | `core/react_ui_backend/compare/` | small | Sonnet | B1 |
| B2 | Route `webui-generations` | `react_ui_backend/webui_generations.go` | 53 | Sonnet | B1, T1 |
| B3 | Route `upload` | `upload.go` | 62 | Sonnet | B1, T1 |
| B4 | Route `gradio-file-proxy` | `gradio_file_proxy.go` | 80 | Sonnet | B1, T1 |
| B5a | Data endpoints and page edits for `pages/outputs/*` | `data_outputs.go` + pages | 3 pages | Sonnet | B1, T1 |
| B5b | Data endpoints and page edits for `index`, `db`, `bark/voices`, `extensions/[name]` | `data_*.go` + pages | 4 pages | Sonnet | B1, T1 |
| B6 | Route `gradio/[name]` | `gradio.go` | 614 | Opus | B1, T1, B4 |
| R1 | Final adversarial review | none | n/a | Opus | everything |

Reviewers: Haiku work is reviewed by Sonnet, Opus work by a fresh Opus that did not write it, and Sonnet work gets an Opus spot-check for B3 and I1.

## Prompt (paste this to the orchestrator)

```
You are the orchestrator for the TTS WebUI Go-core port. Work fully autonomously: do not
stop for the user between waves. Run every task as a detached top-level subagent (not
nested), each in its own git worktree and branch. Do not implement tasks yourself.

GOAL
Port installer_scripts to Go (core/installer_scripts/, one Go file per original script,
same names as the originals; do not rename or refactor) and move react-ui's server-side
work into a Go backend (core/react_ui_backend/) that sits IN FRONT of Next.js as a reverse
proxy. server.py stays Python. Nothing is cut over by default: the old scripts and Next
routes remain the default; Go paths sit behind switches the user flips later.

BRANCHES
Integration branch: core/go-port (already cut from core/rings-1-2; push it if missing).
Each task works on go-port-work/<task-id> from the current core/go-port tip (Git refuses
core/go-port/<task-id> because core/go-port is itself a branch), and opens a PR into core/go-port. After its review passes and CI is green you may squash-merge that
PR into core/go-port (the repo allows squash only) so dependent tasks can build on it.
NEVER touch main, never merge into main, never tag, release, force-push, or delete branches.
Do not modify any user install folder. Leave no process running.

READ FIRST
documentation/core-rewrite/PLAN.md, DECISIONS.md, core/README.md, and the task table in
documentation/core-rewrite/ORCHESTRATOR.md (this file). Wave 0 produces
documentation/core-rewrite/CONTRACTS.md; every later task obeys it.

COMMUNICATION (do not use chat for progress)
Comms directory: ~/Desktop/tts-comms/   (create it; on Windows %USERPROFILE%\Desktop\tts-comms)
- Every session writes ONLY its own file: tts-comms/<task-id>.md (orchestrator: ORCH.md).
- File format: first line "STATUS: queued|running|blocked|review|done|failed", second line
  "UPDATED: <ISO time>", then a short log (newest last): what was done, PR link, checks
  (pass/fail/not run), what was verified by a real run, what was NOT verified, open issues.
- Update the file at start, at each milestone, and at the end. Not more often.
- The orchestrator reads all files about every 10 to 15 minutes, acts on blocked/failed
  entries, and keeps ORCH.md current (wave, running tasks, queue). No one else messages.

AUTONOMY AND GATES (internal; do not wait for the user)
- Wave 0: C0 (Opus), H0 (Haiku). A fresh Opus reviews CONTRACTS.md for gaps and
  contradictions against the real code; fix and continue.
- Wave 1: B1 (Opus), T1 (Sonnet), I1 I2 I3 I5 (Sonnet), I4 (Haiku).
  GATE before Wave 2: a reviewer Opus verifies the B1 proxy in a real browser (page loads,
  a generation streams, an upload works, Stop leaves nothing running). If it fails, send it
  back to the author, then re-review. Only then start Wave 2.
- Wave 2: B2 B3 B4 B5a B5b (Sonnet), I6 (Opus), I7 (Sonnet).
- Wave 3: B6 (Opus).
- Wave 4: R1 final adversarial review of everything on core/go-port.
- Max 6 agents running at once. A failed task: retry once with a fresh agent and the failure
  notes; if it fails again mark it blocked, record why, and continue with independent tasks.
- Do not ask the user questions. Choose the most reversible option and record it in ORCH.md.
- When every task is as done as it can be (done, or blocked with a stated reason), write
  tts-comms/FINAL.md and stop. The user reviews once, at the end.

RULES FOR EVERY TASK
1. Disjoint files. Each task edits only its own output path. B1 owns the Next port change in
   core/internal/launcher/supervisor.go and the proxy switch. After B1 merges, shared launcher
   code (core/internal/launcher/*) is edited ONLY by I7. BL may land a minimal launcher bug fix
   if a real install cannot proceed without it, and must log it. Others report needs in their
   comms file instead of editing it. Route claims register from each route's own file.
   Go file names never end in a GOOS or GOARCH word such as _js or _windows unless a build
   constraint is intended.
2. The original is the spec. Port faithfully, keep names close to the original, no new
   features, no refactors.
3. Real-run verification is mandatory:
   - Installer files: run the Go port and the original script against separate scratch
     installs under a folder with no spaces (e.g. C:\scratch\<task>-test) and diff marker
     files, env, created folders, exit codes.
   - Routes: run the real Next route and the Go route side by side with the T1 harness; test
     a large upload and a streaming response where relevant; click the real UI in a browser.
   - Past runs reported success while the real UI was broken. If you did not run it, write
     "not run". Never write "verified" for anything you did not run.
4. Before every PR: gofmt -l is empty; go vet, go test and CI's committed-asset check pass.
   Stop only processes you started, by PID.
5. Commit each logical change separately; messages say what the diff does.
6. No board. Each task's file in the comms directory states what was and was not verified.

MODELS
Use the model in the table. Reviews: Haiku work by Sonnet; Opus work by a fresh Opus that did
not write it; Opus spot-check of Sonnet work for B3 and I1. Reviewers try to break the work by
running it.

TASKS
C0  CONTRACTS.md: marker files (.gpu .preflight_ok .git_version), env vars the Python app
    and init_app.js expect, the step-runner interface (run a command with env, stream a log,
    report exit), the route-claim table (path -> Go handler or pass to Next), auth/origin
    policy for port 3000, and the side-by-side test protocol.
H0  Inventory (see table).
I1  init_mamba.go   I2  root.go   I3  diagnostic.go
I4  start_app.go activate.go open_terminal.go run_extension_manager.go init_app.go
I5  apply_database_config.go
I6  initialize_app.go and init_app_nodejs.go (the init_app.js flow). Keep pins in one place. Port,
    do not redesign. Record which helper modules are deliberately left out.
I7  Per-step switch (script default / Go), marker-file compatibility tests, no default change.
B1  core/react_ui_backend/proxy.go: Go listens on 3000, reverse-proxies everything to Next on
    an internal free port. FlushInterval -1 for SSE/streaming, no short timeouts, large
    uploads OK, /_next/* and /_next/data/* passed through, Host/Origin preserved per
    CONTRACTS.md. The supervisor starts Next on the internal port only when the proxy switch is
    on; with it off, Next listens on 3000 as today. Claims nothing yet.
T1  Compare harness (see table).
B2  webui-generations: keep the allow-listed directories and dotfile denial; add traversal tests.
B3  upload.   B4  gradio-file-proxy.
B5a pages/outputs/*   B5b index, db, text-to-speech/bark/voices, extensions/[name]:
    replace getStaticProps/getStaticPaths filesystem reads with Go endpoints the pages fetch.
B6  gradio/[name] (614 lines).
R1  Fresh scratch install through the exe with the switches ON; click every step and the UI;
    compare against the script/Next path; Stop leaves nothing running. Try to break it.

FINAL REPORT (tts-comms/FINAL.md)
Table: task, model, PR, checks (pass/fail/not run), what was verified by a real run, gaps.
List everything not run or blocked. State the merge state of core/go-port versus main (never
merged into main).
```
