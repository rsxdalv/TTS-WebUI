# Core rewrite plan (side-by-side)

The TTS WebUI core (bootstrap/installer, process supervisor, reverse proxy, UI host, updates) is
being rewritten in Go **next to** the existing app. The old path stays runnable at every commit until
cutover. Decisions already made are in [DECISIONS.md](DECISIONS.md); handoff notes per checkpoint are in
[notes/](notes/).

Agents: run this with the `core-rewrite` custom agent (`.github/agents/core-rewrite.agent.md`).
One checkpoint per session.

## Status

`ready` = dependencies done, can start · `waiting` = dependencies not done · `in progress` · `built` = code+unit tests done, no smoke · `reopened` = gaps found; treat as `ready` · `done` · `blocked`

| Id | Checkpoint | Lane | Depends on | Status |
|---|---|---|---|---|
| C0 | Go toolchain, `core/` scaffold, CI | G | none | done |
| C1.1 | Vendor tts-webui-ignition into `launcher/` | L | none | done |
| C1.2 | Platform adapter in launcher UI | L | C1.1 | done |
| C2.1 | `server`: local HTTP server, embedded UI, security, SSE | G | C0 | done |
| C2.2 | `procs`: spawn, log capture, tree-kill, no orphans | G | C0 | done |
| C2.3 | `paths` + `state`: install root, marker files, atomic JSON | G | C0 | done |
| C2.4 | `gpu`: detection mapped to installer GPU labels | G | C0 | built |
| C2.5 | `preflight`: port of ignition preflight checks | G | C2.3 | done |
| C2.6 | Browser auth for SSE, media and downloads (no custom headers) | G | C2.1 | built |
| C3.1 | Launcher on Go, legacy-compat mode (same steps, same markers) | J | C1.2, C2.1, C2.2, C2.3, C2.5, C2.6 | done |
| C3.2 | Running app under the Go supervisor | J | C3.1 | done |
| C4.1 | `tools`: pinned uv / ffmpeg / MinGit with checksums | G | C2.3 | built |
| C4.2 | `env`: uv Python + main venv + torch variant | G | C4.1, C2.4 | done |
| C4.3 | App runs from uv env (`--env=uv` next to `--env=conda`) | G | C4.2, C3.2 | done |
| C5.1 | Engine manifest schema + loader over `extensions.json` | G | C2.3 | done |
| C5.2 | Python-venv engine install (port of `venv_setup_wrapper`) | G | C5.1, C4.2 | done |
| C5.3 | `router`: reverse proxy replacing `gradio_proxy_tree` | G | C2.1 | done |
| C5.4 | Engine lifecycle: on-demand start, health, idle stop, cancel | G | C5.1, C2.2 | done |
| C5.5 | Legacy Gradio host + proxy extensions run as core engines | J | C5.2, C5.3, C5.4, P3 | done |
| C6.1 | Core API replacing Next.js API routes | G | C2.1, C5.3 | done |
| C6.2 | `ui/` Vite SPA scaffold, embed pipeline, `--ui-dev` | U | C2.1 | done |
| C6.3 | Port pages, one group per checkpoint (see list) | U | C6.1, C6.2 | done |
| C7.1 | Core self-update | G | C2.3 | done |
| C7.2 | Blue/green engine updates | G | C5.2 | done |
| C7.3 | App (Python host) distribution without git pull | J | C4.3, D-open-3 | done |
| C7.4 | Catalog fetch, cache, schema versioning | G | C5.1 | done |
| C8.0 | Research: audio.cpp server lifecycle and cancel | R | none | done |
| C8.1 | Native engine kind (asset by OS/arch/GPU, verify, launch) | G | C5.4, C4.1 | done |
| C8.2 | audio.cpp engine + cancel | G | C8.0, C8.1 | done |
| C9.1 | Start scripts and release zip launch the Go binary | J | M1–M4 | waiting |
| C9.2 | Remove micromamba, Node installer, Neutralino build | J | C9.1, human only | waiting |
| P1 | Drop PostgreSQL from the conda env spec | P | none | done |
| P2 | Extract `tts-webui-sdk` Python package | P | none | done |
| P3 | `setup_proxy_extension` delegates to core when `TTS_WEBUI_CORE_URL` is set | P | C5.4 | done |
| P4 | Move in-process extensions to `"proxy": "native"`, in batches | P | P2 | reopened |
| P5 | Torch-optional main host (lazy imports) | P | P4 | done |

Lanes: **L** launcher UI (TypeScript) · **G** Go core · **U** new UI (TypeScript) · **P** Python · **R** research ·
**J** join (touches several lanes, run alone).

Milestones (human sign-off each):
- **M1** after C3.2: the Go launcher can replace Neutralino as the default launcher. Path: C2.4, C2.5, C2.6, C3.1, C3.2.
- **M2** after C4.3: uv mode can be the default for new installs.
- **M3** after C5.5: the Python proxy tree is retired behind a flag.
- **M4** after C6.3: the core serves the whole UI; Next.js leaves the runtime.

## Reopened after run 1 — Wave R2.1 (2026-10-29)

Corrective wave on branch `core/r2-wave-1`. See per-checkpoint notes for details.

- **C2.4** (`built`): GPU labels were old short strings (`nvidia`, `amd`, etc.). Fixed to exact PyTorchChoice
  strings; `Parse()` added; all consumers updated; test uses real `.gpu` file contents. Status `built` because
  no binary smoke run yet.
- **C2.6** (`built`, new): Token travels only in `X-TTS-Token` header. `EventSource`, `<audio src>`, `<img>` and
  download links cannot send custom headers. Added `SameSite=Strict`, `HttpOnly` cookie set on index.html serve;
  `tokenGuard` accepts cookie OR header; Origin guard still applies.
- **C4.1** (`built`): Every SHA256 was empty (skipped verification). Filled: uv 0.13.6, ffmpeg n9.0, MinGit 2.49.0.
  Added `TestPinnedSHA256NonEmpty` as a build-time gate. Status `built` (no real download run; HEAVY_CHECKS=no).
- **C8.0** (`done`): Was based on wrong repo (`rsxdalv/audio.cpp`, private). Corrected to `0xShug0/audio.cpp`
  (Apache-2.0, v0.9.3). All API, cancel, streaming, asset naming facts sourced from the correct repo.
- **P4** (`reopened`): Five extensions had `"proxy": "native"` added without real audio generation to verify.
  Reverted all five. Re-application requires HEAVY_CHECKS=yes or human verification.

## Parallel work

Checkpoints in different lanes touch disjoint files and can run in parallel sessions, each on its own branch
(ideally its own worktree). Within lane G, checkpoints that create different `internal/` packages are also
disjoint. `J` checkpoints run alone. Suggested waves (2–3 sessions at a time is plenty):

1. C1.1 · C2.1 · C2.2 · C2.3 · C2.4 · P1 · P2 · C8.0
2. C1.2 · C2.5 · C4.1 · C5.1 · C5.3 · C6.2
3. C3.1 → C3.2 (join, alone) · then M1
4. C4.2 · C5.4 · C7.1 · C7.4 · P3
5. C4.3 · C5.2 · C6.1 · P4 batches
6. C5.5 (alone) · C6.3 page groups in parallel · C8.1
7. C7.2 · C7.3 · C8.2 · then C9.x

`main.go` and `core/README.md` are shared by all G checkpoints. To avoid conflicts, each checkpoint adds its
subcommand in its own file (`cmd/tts-webui/cmd_<name>.go`, registering itself from `init()`, see `cmd_version.go`)
and never edits `main.go`; it appends a section to the README rather than rewriting it. Status-table edits are
one line per checkpoint, so parallel branches merge cleanly.

## Running old and new side by side

- **Separate install roots.** The core takes `--root <dir>` (default: the directory of the binary's parent
  app checkout). Fresh-install tests use a scratch root, e.g. `D:\tts-core-test`. Never point a test run at
  the developer's working install unless the checkpoint is explicitly about legacy-compat mode.
- **No port collisions.** Legacy uses 3000, 7767, 7769, 7770, 7771, 7778 and 27771+. The core's public port is
  [DECISION D-open-1]; every internal port comes from the OS.
- **Legacy-compat mode first, native second.** Each area is first re-driven by Go doing exactly what the old
  path did (same commands, same marker files), which gives a clean parity check. Only then is it replaced
  by the native version, selectable by flag (`--env=conda|uv`, `--proxy=python|core`), until a milestone
  flips the default.
- **Parity checks** are part of the definition of done when a checkpoint replaces old behaviour. Kinds:
  - *State parity*: same marker files and contents (`installer_scripts/.gpu`, `.git_version`, `.preflight_ok`).
  - *Env parity*: `uv pip freeze` of the new env vs `pip freeze` of the old, diff checked in with only
    expected differences explained.
  - *HTTP parity*: the same requests through the Go router and through the Python proxy tree give the same
    status, headers that matter and body (scripted, recorded in the notes).
  - *UI parity*: the same page in old and new UI, screenshot pair in the PR.

## Checkpoints

Each checkpoint lists: goal · files it may touch · CLI demo · parity · done when. Every checkpoint also has
the general definition of done from the agent file (CI green, reviewer run, PLAN updated, handoff note).

### C0 — Go toolchain and scaffold (done)
Go 1.27, module `github.com/rsxdalv/TTS-WebUI/core`, `cmd/tts-webui` with `--version`, `internal/version`,
`internal/webui` (go:embed), `.github/workflows/core.yaml` (gofmt, vet, test, 6-target `CGO_ENABLED=0` matrix).
Stripped binaries are about 1.6–1.7 MB. See [notes/C0.md](notes/C0.md).

### Phase 1 — Vendor the launcher (lane L)

tts-webui-ignition (Neutralino 6.7 + Vite 7 + React 19 + Tailwind 4, about 2,200 lines of TSX) is the modern
launcher. It is a UI over the legacy installer: a waterfall of 7 steps (download, preflight, mamba, gpu,
install, update, run) driven by marker files, with every action done through Neutralino shell calls.
The plan keeps its UI and moves its logic into Go.

**C1.1 Vendor ignition.** `git subtree add --prefix=launcher https://github.com/rsxdalv/tts-webui-ignition main`
(history preserved, see D7). Add a `launcher.yaml` workflow that builds the Vite app and the Neutralino
binaries from the monorepo, equivalent to ignition's `build.yml` (no publishing yet). Fix the README (the
`WEBUI_ROOT` vs `VITE_WEBUI_ROOT` mismatch, stale "must be pre-installed" text).
Touches: `launcher/**`, `.github/workflows/launcher.yaml`. Done when: the workflow builds all 7 binaries;
a locally built Windows binary installs and launches the app as the standalone ignition does.

**C1.2 Platform adapter.** Move every Neutralino call behind `launcher/vite-src/src/platform/` with an
interface covering what is used: exec, spawn (with output events), kill, file stat/read/write/mkdir, OS and
cwd. One implementation: `neutralino`. Pure refactor, no behaviour change. Steps should call
*step-level* operations (`runPreflight`, `selectGpu`, `install`, …) rather than raw shell strings where that is a
small change, because the Go backend will implement those operations, not a generic shell.
Scout task: re-inventory every `Neutralino.*`, `NL_*` and `__neutralino_globals` use with `path:line`.
Done when: no Neutralino import or `NL_*` global outside `platform/`; launcher builds and runs as before.

### Phase 2 — Go core primitives (lane G, packages are file-disjoint)

**C2.1 `internal/server`.** HTTP server bound to `127.0.0.1` only. Serves the embedded UI (`internal/webui`).
Security from day one, because this server will start processes: a random per-run token injected into the
served HTML and required on `/api/*`; reject requests whose `Host` is not `127.0.0.1:<port>`/`localhost:<port>`
(DNS-rebinding) and cross-origin `Origin`s. `/api/health`, an SSE helper for streams. `tts-webui serve`.
Done when: tests cover token, Host and Origin rejection; `serve` opens the placeholder page.

**C2.2 `internal/procs`.** Start a process with cwd, env and args (no shell strings); capture stdout and stderr
into a ring buffer per process and fan out to subscribers; graceful stop then tree kill. Windows: Job Object
with kill-on-close; Unix: process group. `tts-webui run -- <cmd>`.
Done when: a test spawns a child that spawns a grandchild, kills the parent tree and proves no survivors on
Windows and Linux; a test proves the core exiting (even by crash) takes the children down on Windows.

**C2.3 `internal/paths`, `internal/state`.** Resolve the install root and well-known paths
(`installer_scripts/`, `installer_files/`, `.venvs/`, `outputs/`, …). Read and write the legacy marker files
exactly as the installer and ignition do. Atomic JSON state with a `schema` field for the core's own state
(under `installer_files/core/`). Done when: tests use golden copies of real marker files.

**C2.4 `internal/gpu`.** Detect NVIDIA (`nvidia-smi`), AMD ROCm (Linux), Apple Silicon, Intel XPU, else CPU,
and map to the installer's labels (`PyTorchChoice` in `installer_scripts/js/initializeApp.js`, identical in
ignition's `GPUSelectStep.tsx`). Detection *suggests*; the user still chooses. `tts-webui gpu detect --json`.
Done when: parsing is unit-tested with recorded command outputs; the label set matches the legacy list exactly.

**C2.5 `internal/preflight`.** Port ignition's `PreflightChecksStep.tsx` checks: spaces in path, System32,
exFAT, long paths (offer the `.reg`), conflicting conda, MSVC via vswhere (winget install offer), gcc on
Linux/macOS. Each check returns status + message + optional fix action; fixes are separate calls that the UI
must request explicitly. Note ignition issue #1: the winget `--add` argument fails on current winget, so
get the command right here. `tts-webui preflight --json`.
Parity: on the same machine, same pass/fail per check as ignition. Writes `.preflight_ok` like ignition.

### Phase 3 — Launcher on Go (join)

**C3.1 Launcher in legacy-compat mode.** Add a `core` implementation of the launcher's platform adapter that
talks to the Go server over HTTP + SSE using **typed step endpoints** (`POST /api/steps/preflight`,
`/api/steps/gpu`, …), never a generic "run this shell string" endpoint. Go implements each step doing what
the legacy path does today: download the release zip (with TLS verification, unlike `curl -k`), preflight,
`init_mamba`, write `.gpu`, `micromamba run node installer_scripts/init_app.js`, git update check.
The Go binary serves the launcher's Vite build (copied into `internal/webui/assets` at build time) and opens
the browser: `tts-webui launch`. The Neutralino build keeps working from the same source.
Fix the known ignition bugs on the Go path and list them in the notes: Windows-only `\\` path in
`isGitUpdated` (update never runs on macOS/Linux), mamba step "done" when only the directory exists
(check for the micromamba binary), backslash paths in the extension-manager command, `start` via spawn.
Parity: state parity (markers, `installer_files/env`) between a Neutralino install and a Go install into two
scratch roots. Done when: a fresh Windows install via the Go path completes and the app starts.

**C3.2 Running app under the supervisor.** The "run" step: Gradio (`server.py --no-react`), React UI, extension
manager and the console server run under `procs`, logs streamed to the launcher UI, all children stopped when the
core exits. Resolve the port 7771 collision (installer console server vs extension manager). Replace the
500 ms `/poll` polling with SSE where the Go side owns the data. Done when: start/stop/restart from the UI,
no orphans after closing the browser and stopping the core. **→ M1.**

### Phase 4 — uv replaces micromamba (lane G, side-by-side env)

**C4.1 `internal/tools`.** Download pinned uv, ffmpeg (static build) and, on Windows, MinGit (uv needs git for
`git+https` requirements) into `installer_files/tools/`, verified against checksums pinned in the source.
`tts-webui tools ensure`. Done when: re-runs are no-ops; a corrupted download is detected and replaced.

**C4.2 `internal/env`.** `uv python install` of the pinned Python (3.10.11 today); create the main venv with the
torch variant for the chosen GPU label (same index URLs as `initializeApp.js`). `UV_CACHE_DIR` on the same
volume as the venvs so uv hardlinks (D3); `UV_LINK_MODE` left at default, warn if uv reports a copy fallback.
`tts-webui env create main`. Parity: env parity against the conda env (expected differences listed).

**C4.3 App from the uv env.** Install the same pip packages `init_app.js` installs, then run the app from the uv venv:
`tts-webui launch --env=uv`. `--env=conda` stays the default. ffmpeg and git from `tools` on the app's PATH.
Done when: on Windows and Linux a builtin tab, one in-process extension and one proxy extension each produce audio
in uv mode. **→ M2.**

### Phase 5 — Engines, supervisor and router

**C5.1 Engine manifest.** Extend extension entries with an optional `engine` block (D5 sketch):
`kind` (`python-venv` | `native` | `remote`), install recipe, `launch`, `health`, `api` (OpenAI-speech path,
params schema), optional `ui` (Gradio iframe), `cancel` (`disconnect` | `endpoint`), `keep_alive`.
Loader reads `extensions.json` + `extensions.external.json` and derives manifests for current entries
(`"proxy": "native"` → `python-venv`). JSON Schema checked in. `tts-webui engine list`. **[DECISION D-open-2]**
on the schema before merging.

**C5.2 Python-venv engines.** Port `venv_setup_wrapper` from `tts_webui/utils/pip_install.py` (torch,
torchcodec, xformers, requirements + compat pins) into `internal/engine`, installing into versioned dirs
`.venvs/<pkg>@<version>` with a pointer file. Parity: env parity against a venv created by the Python path.

**C5.3 `internal/router`.** Replace `tts_webui/gradio_proxy_tree`: prefix routing, raw-path preservation
(Gradio `?file=C:/...` URLs must pass intact), hop-by-hop header handling, `x-forwarded-*`, streaming responses,
websockets, long timeouts, cancellation when the client goes away. Tests against an `httptest` fake Gradio
reproducing the cases in `gradio_proxy_tree/handlers.py`. Parity: HTTP parity, Go router vs Python proxy tree.

**C5.4 Engine lifecycle.** Start on demand, free port from the OS, readiness via health check (not port polling),
log streams, stop on idle after `keep_alive`, optional cap on resident engines (VRAM), and a cancel operation that
uses the manifest's `cancel` mode (close upstream connection, or call the endpoint). API:
`GET /api/engines`, `POST /api/engines/{id}/start|stop`, `GET /api/engines/{id}/logs` (SSE).
Done when: tested with a fake engine binary, including crash-and-restart and cancel.

**C5.5 Legacy host as an engine.** The core starts `server.py` as the `legacy-gradio` engine mounted at `/` and
starts proxy extensions itself (P3 makes the Python side ask the core). `--proxy=python|core`.
Parity: every `"proxy": "native"` extension loads through the Go router inside the Gradio UI. **→ M3.**

### Phase 6 — New UI (lane U)

**C6.1 Core API for the UI.** Replace the Next.js API routes: `pages/api/gradio/[name].tsx` (drop; the browser
uses `@gradio/client` against the router, same origin), `pages/api/gradio-file-proxy/[...path].tsx` (router
covers it), `pages/api/webui-generations/[...name].ts` (core serves `outputs/`, `favorites/`, `voices/`… read-only,
path-traversal safe), `app/api/upload/route.ts` (core upload endpoint with size limit). TypeScript types for the
API generated or hand-written next to the Go types.

**C6.2 `ui/` scaffold.** Vite + React + react-router app in `ui/`; shared components moved from `react-ui/src`
(copy, do not delete legacy). Build output copied into `core/internal/webui/assets` by the release build;
`tts-webui serve --ui-dev=http://localhost:5173` proxies UI requests to the Vite dev server for hot reload.
The launcher becomes the `/setup` route here or later (**[DECISION D-open-4]**).

**C6.3 Page groups** (one checkpoint each, file-disjoint, good parallel subagent work): text-to-speech ·
audio-conversion · audio-music-generation · tools · outputs/history/favorites · extensions · settings and misc.
Replace `getStaticProps`/`getStaticPaths` (and `fallback: "blocking"`) with fetches. Drop the dead Postgres
code (`sql.tsx`, `db.tsx`). UI parity screenshots per page. **→ M4.**

### Phase 7 — Updates

**C7.1 Core self-update.** Check GitHub Releases, download, verify checksum + signature (**[DECISION D-open-5]**:
minisign vs cosign, key custody), replace the binary (Windows: rename running exe to `.old`, put the new one in place,
restart), keep `.old` for rollback. Never update while engines are generating.
**C7.2 Blue/green engines.** Build the new versioned venv or native dir next to the old one, health-check, switch the
pointer, garbage-collect old versions after N days. A failed update leaves the working version in place.
**C7.3 App distribution.** Replace `git pull` of the whole repo (**[DECISION D-open-3]**: pinned `tts-webui`
package installed by uv vs. a release zip per version vs. keep git for now). Includes migrating existing git installs.
**C7.4 Catalog.** Fetch the extension catalog, cache it, check its `schema` version, fall back to the cached copy offline.

### Phase 8 — Native engines

**C8.0 Research (no code).** For audio.cpp's server: does it stop computing when the client disconnects? Is there a
job id / cancel endpoint? Is ggml's abort callback wired (`grep abort_callback`)? Streaming format? Concurrency
(queue / parallel / reject)? Model load/unload endpoints and idle unload? Health/readiness? Release assets per
OS/arch/GPU and their naming? If cancel is missing, estimate an upstream patch (cancel-on-disconnect + abort per
decode step). Output: `notes/C8.0.md` + manifest draft. Also confirm the facts from earlier research
(model list, maturity) against the repo itself.
**C8.1 Native engine kind.** Pick the release asset for OS/arch/GPU, download, verify, unpack into a versioned dir, launch.
**C8.2 audio.cpp engine.** Manifest, UI integration, cancel per C8.0. If the server has no cancel, the core falls back to
sentence-chunked requests (stop = do not send the next chunk), with kill-and-restart as the last resort.

### Phase 9 — Cutover (human-gated)

**C9.1** `start_tts_webui.*` and the release zip launch the Go binary; the old `root.ps1` path stays available for one
release. **C9.2** Remove micromamba, the Node installer and the Neutralino build. Human approval required.

### Track P — Python (lane P, touches legacy Python only)

**P1** Remove `postgresql` from the env spec in `init_mamba.*` (never started at runtime; see the codebase report).
Do not touch other packages. **P2** Extract `tts-webui-sdk`: decorators (save wav, metadata, base filename, log,
interrupt, seed), output conventions, `proxy_harness`, `torch_load_patch`; `tts_webui` re-exports for compatibility so no
extension breaks. **P3** When `TTS_WEBUI_CORE_URL` is set, `setup_proxy_extension` asks the core to start/stop the
extension and uses the router URL instead of spawning it. **P4** Move the in-process extensions to
`"proxy": "native"` in small batches (the extensions live in their own repos; this repo changes `extensions.json`),
each batch verified by generating audio. **P5** Make torch optional in the main host (lazy imports in
`torch_load_patch`, `torch_clear_memory`, seed decorator).

## Open decisions (human)

| Id | Question | Recommendation |
|---|---|---|
| D-open-1 | Core public port | A fixed default that collides with nothing legacy uses (e.g. 7700), overridable by flag |
| D-open-2 | Engine manifest schema | Review the C5.1 draft before merge |
| D-open-3 | How the Python app is distributed after git pull | Pinned package installed by uv into the main env |
| D-open-4 | Launcher UI: separate Vite app or `/setup` route of `ui/` | Merge into `ui/` during C6.2 |
| D-open-5 | Update signing | minisign, key held by the maintainer, public key embedded in the binary |
| D-open-6 | Python version for new installs | Keep 3.10.11 until P4 is done, then per-engine |

## Model use (Copilot)

- **Sonnet** (orchestrator, implementers, reviewer): design, Go code, tests, reviews.
- **Haiku** (`scout` agent): read-only inventories, call-site lists, summaries of large legacy files, fixture tables.
  Never Go implementation, never decisions. Good fits: C1.2 Neutralino inventory, C2.5 check list from
  `PreflightChecksStep.tsx`, C5.2 step list from `pip_install.py`, C5.3 case list from `handlers.py`,
  C6.3 per-page inventory of API calls and `getStaticProps`.
