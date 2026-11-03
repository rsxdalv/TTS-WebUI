# Go core rewrite — ring plan

Convert TTS WebUI's launcher and runtime to a Go core, one working ring at a time, keeping uv venvs with hardlinks.
Decisions are in [DECISIONS.md](DECISIONS.md). What was built before this plan, with its detailed gap lists, is in
[archive/PLAN-runs-1-4.md](archive/PLAN-runs-1-4.md). Read that only for the ring you are on.

## How rings work

- A ring is one conversion you can use. It has a demo you can run yourself.
- A ring is `done` only when **the human says so at its demo gate**. Agents report `ready for demo`, never `done`.
- From sign-off on, the human uses the new path, and later rings run on top of it. If a ring is hollow, the next
  one breaks at once. That is what keeps them honest.
- Only the current ring gets work. Code for later rings is `parked`: it keeps compiling and passing tests, with
  no fixes or additions until its ring comes up. A ring may also decide to delete parked code.

## Rings

| # | Ring | Status | User-visible result |
|---|---|---|---|
| 1 | Launcher | **current** | A fresh real install through the Go launcher UI |
| 2 | Supervisor | **current** (shares code with ring 1) | Start, stop and restart the whole app from the launcher, with logs and no orphans |
| 3 | uv env | next | The unchanged `server.py` runs from a uv venv on a clean machine, with hardlinks |
| 4 | `server.py` | parked | The Gradio host as a supervised engine, with the Go router replacing the Python proxy tree |
| 5 | Core extensions | parked | One extension at a time in its own venv, each generating real audio |
| 6 | Native engines | parked | audio.cpp installed, running, cancellable |
| 7 | React UI | parked, very late | Decide then whether a port is worth it. Until then Next.js is a supervised process |

Updates (self-update, blue/green, app distribution, `C7.x`) are pulled in when a ring first needs them. Cutover
(`C9.x`) is human-only.

## Current: rings 1 and 2

### Where it stands (2026-11-03)

At `4a4a362` on `core/rewrite`. vet, all go tests, m1.ps1 (PASS), and 10 Playwright tests (PASS):
- Diagnostic button added to LaunchStep; `POST /api/app/diagnostic` + stream.
- `app/start` now guards GPU label in all modes (conda + uv).
- `install` step now 400s if mamba or GPU not yet done.
- m1.ps1 now covers restart (link reappears) and diagnostic stream.
- All 5 Run-3 review items verified PASS.

**Awaiting demo gate (human run, ~5 min each).** See demo instructions below.

### What is left

1. Commit the smoke work and get `m1` passing to the rules below, on the current code.
2. Get the Playwright script passing: every step through Launch, app link appears, Stop works, Restart works, reload
   mid-step reattaches, a root with a space shows the failed check, a stub that exits 1 shows a failure, clicking Launch
   twice does not start a second copy.
3. Fix what those find. Check these Run 3 review items were really fixed, by behaviour, not by reading the commit
   message: Diagnostic is available as in `ReactUIStep.tsx`; `run` no longer exists anywhere in the UI; the 7771 collision
   is resolved; a failed process is visible in the UI; `env create main` without `--gpu` fails before installing Python.
4. Reviewer pass: click through from a fresh fixture and try to break it.
5. **Demo gate, ring 1.** The human does a real install into a new scratch folder with no spaces in its path:
   real mamba, real install, GPU choice. Compare marker files with a Neutralino install.
6. **Demo gate, ring 2.** On that install: Launch, Gradio answers, React UI opens on port 3000, extension manager opens,
   Stop leaves nothing running, Restart works, killing the launcher leaves nothing running.

Real-install bugs found at the gates are fixed in the same ring.

### Out of scope

uv mode, torch pins, tool table gaps, audio.cpp, the router, `ui/`, updates.

## Later rings

Each coordinator expands its ring at the plan gate. Old checkpoint ids point into the archive.

**Ring 3, uv env** (`C4.1`, `C4.2`, `C4.3`).
Goal: the same old `server.py`, unchanged, running from `.venvs/main`.
- Conda also supplied Node, `ninja` and ffmpeg (`init_mamba.sh:162`), and Node has no pin yet. Add it with real checksums.
- The launcher's install and update steps always call micromamba; they must follow `--env`.
- Install the same packages as `init_app.js`, using one torch table matching `initializeApp.js`.
- One uv version and one cache across the core and `tts_webui/utils/pip_install.py`, on the same volume as the venvs.
- Fix: macOS has no ffmpeg entry, windows/arm64 has no MinGit entry.

Demo: fresh scratch folder, `launch --env=uv`, generate one clip with a builtin tab, show the `pip freeze` diff against
conda, and prove with `fsutil hardlink list` that a torch file in two venvs is one file.

**Ring 4, `server.py`** (`C5.3`, `C5.5`, `P3`, `P5`).
Demo: `--proxy=core`, builtin tabs work, and one proxy extension's page loads through the Go router.

**Ring 5, core extensions** (`C5.1`, `C5.2`, `C5.4`, `P2`, `P4`).
One extension per slice, in an order the human picks. Each generates real audio before `extensions.json` changes.

**Ring 6, native engines** (`C8.0`, `C8.1`, `C8.2`).
The known gaps are in the archive's "Still open after R2.3b". Demo: install, generate, cancel mid-generation.

**Ring 7, React UI** (`C6.x`). The first gate is whether to do it at all.

## Verification rules

- **Test what the user does.** Smoke scripts make the same calls the UI makes. Browser scripts click the real UI served
  by the real binary. If the UI and the script take different paths, that is a failure.
- **A check must be able to fail.** Nothing is excluded by a before/after list or a process-name filter. No INFO,
  NOTE or "expected" for a failed check. A warning is not a pass. If a requirement cannot be met, report it failed.
- **Legacy contracts use real legacy data.** Marker files and paths come from what the legacy scripts really write, not
  from values the stubs write themselves.
- **The stubs stop at the boundary.** Fake `micromamba`, `python` and scripts are fine; fake core packages are not.
  Unit tests with injected runners do not count as a smoke.
- **No placeholders on a user path:** no TODO, empty checksum, endpoint the UI calls that does not exist, or silent
  fallback for unknown input.
- **Research cites what it read.** An unreachable source means the claim is dropped or the item is blocked.
- **Commit messages say what the diff does.** A planned fix that didn't work is stated as such.

### M1 smoke (rings 1 and 2)

`core/scripts/smoke/m1.{ps1,sh}` against a fixture with stub scripts, exiting non-zero on any failure:
- The embedded JS bundle contains `/api/steps` and no `Neutralino`.
- Every step is driven over HTTP with only the cookie from `GET /`.
- Each stream delivers a line printed by the stub, then `event: done`.
- Marker files match golden copies of what the legacy flow writes.
- The app starts as the Launch button starts it. A fake `python` answers HTTP on its port 5 s later.
- After `app/stop`, no descendant of the launcher remains.

## Rules

- Legacy stays runnable at every commit. New behaviour stays behind flags until the human flips it.
- Never touch the developer's install state in the checkout (`installer_files/`, `.venvs/`, `outputs/`, `favorites/`,
  `voices/`, `config.json`, `.env`, `data/`). Installs go in a scratch folder under the OS temp directory.
- Never push to `main`, merge the PR, force-push, publish a release or tag, or generate a real signing key.
- Work on `core/rewrite`, with draft PR #712. A short-lived branch per slice is optional.
- `core/**` follows `.github/instructions/core-go.instructions.md`.
