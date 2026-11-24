# platform/gradiohost: the Gradio host as an entry (P3)

Decisions D23, D24, D29 of [PLAN.md](../../../../documentation/core-rewrite/PLAN.md); item P3. This file says what was
built, what differs from the plan, and what was run. What was not run is at the end.

## What it is

| Piece | What |
|---|---|
| `platform/gradiohost` | The host as a `Platform`: `.venvs/gradio-host` made by the uv platform (`pip` and `requirements.txt`, torch on the default row of the GPU choice). `Command("python")` is that venv's python run directly. Tab installs are `pip install ... gradio==5.49.1 -c .venvs/gradio-host/constraints.txt`: the constraints file holds the row's torch pins (`torch==2.11.0+cpu`, `torchaudio`, xformers) so a tab's requirements cannot move torch (the old Go pip path had no pin). Not ready until `entry.json` is there; says "install it from the Extensions page". |
| `platform/detect` | `Detect` returns one platform that **chooses at each call**: `system` with `--system-python`; `legacy-env` when the folder has the legacy env directory (an existing install is adopted, unchanged); otherwise the host. A folder whose env the legacy installer step creates while the launcher is open becomes legacy-env from then on. `detect.Host(p)` is non-nil only on the host. |
| `extcatalog` | `Options.Host`, `Options.AppRunning`. The host is a synthetic entry of the requires graph (requires `uv`); a tab requires it. Plan = uv, host, tab (one GPU question for the first step). Removing it is refused while an installed tab needs it (built-in tabs do not count) and while the app runs. `gradio-host` is not in `extensions.json`. On legacy-env or system, install and remove answer "managed outside this app". With no host, no tab is installed and there is no pip warning. |
| launcher | The Extensions list has a `gradio-host` row (`kind: host`) only on the host platform. With no host, the router's `/` answers a page that says so and links to the Vite UI instead of a 502 (the port stays open: engine pages and extension pages use it), and the browser opens the Vite UI. |
| `supervise` | `StrictGPULabel`, `SkipGPUCheck`, `readGPULabel`, `checkGPULabel` and the status `Warning` are deleted (D29). A static front is no longer given to `Platform.Command`. |

## Differences from the plan

- The plan said "in-process tabs install into its venv with the torch pin kept". Done with pip and a constraints file,
  not uv: pip is seeded in the venv so `pip list` (the Settings page), `latest-version` and the dependency check run
  unchanged. A tab install does not hardlink.
- The Gradio pin (`gradio==5.49.1`) is still a pip argument, not an entry property.

## Vite menu (step 7)

`GET /api/go-data/host-status` (`core-host`, read only) forwards to the control API's `/api/host/status`:
`{installed, installable, detail, launcherPort}`. `installed` is the platform's `Status().Ready` (true on legacy-env and
system, true on the host once its venv is complete, not "Gradio is running"). The menu and the command palette
(`useMenuRoutes` in `Header.tsx`) leave out the pages that call `/api/gradio` (`/pipeline`, `/text-to-speech`,
`/audio-music-generation`, `/audio-conversion`, `/extensions/*`, `/tools/api_config`) only when the answer says not installed,
and show a notice with a link to `http://<this host>:<launcher port>/` (the launcher is told its port by `WithLauncherPort`).
The model cards on the home page and the section pages say "Needs the Gradio host" instead of "Run". While the answer is
loading, or if asking fails, the whole menu shows (a failed question hides nothing).

## Changes found by running it

- Tabs that the host's own `requirements.txt` installs (demucs, mms, vocos, seamless_m4t, vall_e_x, openai_tts_api,
  log_viewer) counted as dependants, so "remove the extra" was refused with eight names. They are part of the host now
  (`extcatalog.hostBaseTabs`, names compared with `-` `.` `_` folded).
- `PyRNNoise` installs into the host but its tab shows "(!)": the extension imports `librosa` and does not declare it (the
  legacy env has librosa from other packages). Not fixed here (upstream extension); installing AP-BWE brings librosa.

## Verification

Unit tests (Windows, Go 1.27.4): `gradiohost`, `detect`, `extcatalog` (`host_test.go`), launcher (`hoststatus_test.go`, no-host
page), `react_ui_backend` (`core_host_test.go`), supervisor. `gofmt`, `go vet ./...` and `go test ./...` pass in `core`.

Real runs, in the P2 scratch folder (`%TEMP%\p2fresh3`, Windows; `/tmp/p2lin/fresh3`, WSL `tts-linux-test`), launcher built from this
branch, `--no-browser --start`:

| Demo line | Windows | Linux (WSL) |
|---|---|---|
| no host: Vite menu, home cards, notice, `/` page on 7770, Go routes | clicked in the browser pane | `host-status` and `/` by curl, the menu was not opened |
| install the host from the Extensions page: uv venv, `uv pip install -r requirements.txt` on 3.10.11 with torch 2.11.0+cpu, constraints file, restart, Gradio behind the router | clicked; 109 tabs then 108, Gradio page in the pane | by the launcher API (`/api/ext/op/install`); 108 tabs, `pip list` (Settings) against the host venv answers |
| install one tab (PyRNNoise, then AP-BWE): "Requires: gradio-host", tab after restart, torch unchanged | clicked; PyRNNoise tab loaded broken (librosa, above), AP-BWE and then PyRNNoise fine | AP-BWE by the API; tab present, torch 2.11.0+cpu |
| remove: refused with a tab; refused while the app runs; tabs removed; app stopped; host removed; Go pages and Kitten still work | clicked; Kitten opened through `/api/go-data/engines/.../open`, 274 KB clip | by the API; Kitten opened, 274 KB clip; Go routes 200 |
| an existing install on legacy-env | scratch root with a junction to the real `installer_files`; 112 tabs, no broken tab, `Runs on: legacy-env`, no host row, Vite menu complete; `LAUNCHER_REAL_ROOT` `TestRealExtensionProcessAppliesDecorators` passes | scratch root with a symlink to the P1 env; 112 tabs, no broken tab, `legacy-env`, no host row |

## Not done / not run

- macOS was not run.
- On Linux the install, tab and removal steps went through the launcher API (what the page calls), not through clicks, and the
  Vite menu and the launcher page were not opened in a browser there (the Windows browser pane cannot reach WSL port 3000).
- The `LAUNCHER_REAL_ROOT` tests that install (`TestRealPipInstallThroughBridge`, `TestRealBridgeStartsAndStopsExtension`,
  `TestRealNativeExtensionInstallAndStart`) skipped; only the decorator test ran.
- Windows path length of `.venvs/gradio-host` in a deep folder was not tried (the scratch root is short).
- A tab install into the host is a pip install: no hardlinks, and the Gradio pin is still a pip argument.
- `diagnostic.go` (one env), `procscan_other.go` (word `micromamba`), npm for the launcher's React build on the host platform
  (`Command("npm")` is refused there: P4 owns the build; `react-ui/out` must be shipped).
- The launcher address in the menu notice is the launcher port the process was started with; a launcher behind a proxy or on
  another host name is not handled.
- The Setup tab of the launcher still shows the micromamba steps on a new folder (P4).
