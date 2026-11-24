# platform: the seam between the core and the runtime an entry runs on (P1)

Design: [PLATFORM-DESIGN.md](../../../documentation/core-rewrite/PLATFORM-DESIGN.md). Decisions D23 and D24 of
[PLAN.md](../../../documentation/core-rewrite/PLAN.md). This is what was built, what differs from the design, and what
was checked.

## What it is

| Package | What |
|---|---|
| `platform` | The `Platform` interface, `Command`, `Entry`, shared entry install logic (`Entries`: pip into the main env, `.venvs/<pkg>` with uv), `Run` (stream a command's lines), `VenvPython`, `OwnVenvCommand`. |
| `platform/legacyenv` | The micromamba main env under `installer_files/env`, adopted. The only shipped code (with the installer step) that knows `micromamba -p <env> run`, `MAMBA_ROOT_PREFIX` and the Windows argument escaping. |
| `platform/system` | `--system-python`: python, uv and npm from PATH. |
| `platform/detect` | `Detect`: `system` when asked for, otherwise `legacy-env`. |
| `platform/requires` | Names, `Derive`, `Validate`, the resolver (`Graph.Resolve`, `State`, `Dependants`). No catalog, no I/O. |

`supervise`, `piplist`, `extcatalog`, `extprocs` and the launcher (`serverpy.go`, `extensions.go`, `react_ui_mode.go`)
take a `Platform`. `Config.CommandPrefix`, `Options.Run/Python/Env` and `extprocs` `MainPython/BaseEnv` are gone.

## Differences from the design

- **Two methods the design did not list:** `Check` (the dependency check, pip `--dry-run`) and `WithEnv` (a copy whose
  commands also carry the `.env` variables that `startServerPy` computes). `Entry.TorchArgs` is a function so the
  torch table can stay in `extcatalog` until P2 and is read only when an install gets that far (same order of
  messages as before).
- **"No platform" is `legacy-env` with `Status` not ready.** `Detect` never returns a third value: the old text
  (`no Python environment: <micromamba> was not found (the install has not been done)`) is kept word for word because
  the status page and tests show it.
- **An install on a folder with no platform is refused before anything runs** (HTTP 400, `<entry> requires uv (the uv
  platform is not available in this version)`), where it used to start, fail on the missing program and stream
  `Failed ...`. This is the demo line "a new folder says what would be installed". The same refusal applies to a
  pasted entry.
- **Windows escaping now applies to every command through `legacy-env`,** including the supervisor's Gradio child.
  Before only installs were escaped. For plain arguments (`server_core.py --no-react`) it is the identity.
- `ToolDirs` and `ProcessMarkers` were not added (Rob, 2026-11-24): `go_ffmpeg_metadata.go` and `procscan_other.go`
  stay on the grep allow-list.
- `guard_test.go` reworded seven comments and one preflight message (`outside installer_files/` now says `outside the
  install folder`).

## What it keeps unchanged

The torch table, the `.gpu` file and its gate, `gradio==5.49.1` pinning, the install messages and logs
(`installer_scripts/logs/*`), the NDJSON stream of an install, native extensions left behind in `.venvs/<pkg>` on
removal, "installed" for a native extension meaning the directory exists (the P0 observation, fixed in P2).

## Verification

Run on 2026-11-24 from `platform/p1-seam`, Windows 10 (Go 1.27.4) and WSL `tts-linux-test`, in scratch copies made by
`git archive` as in [P0-BASELINE.md](../../../documentation/core-rewrite/P0-BASELINE.md) (`installer_files` a junction or
symlink to the real install, a copy of `.gpu`, `server_port` 17930/17940, `react-ui/out` copied from the P0 scratch).

| Line | Windows | Linux (WSL) |
|---|---|---|
| Start with the real env, all extensions enabled (`POST /api/app/start`) | gradio ready after 8 s, 112 `goodtab`, 0 marked `(!)`, `Huggingface Cache Manager` present, React 3000 answers 200 | ready after 12 s, 112 tabs, 0 `(!)`, HF tab present, React 200 |
| Stop (`POST /api/app/stop`) | public port and 3000 closed, no process of the copy left | both ports closed |
| `TestRealExtensionProcessAppliesDecorators` | PASS | PASS |
| `TestRealBridgeStartsAndStopsExtension` (`LAUNCHER_REAL_PYTHON`) | PASS | PASS |
| `TestRealNativeExtensionInstallAndStart` (Piper, warm cache) | PASS (41 s) | not run (P0 did not either) |
| Extensions list on the real install (`/api/ext/list`) | `platform legacy-env`, ready; 47 entries `gradio-host` ready via legacy-env, 20 `uv` ready via legacy-env | same counts |
| New folder, no `installer_files` | `platform_ready false` with the old message; Bark: `requires uv`, `missing`, `would: the uv platform is not available in this version`; `POST install` refused 400 with that text; no `.venvs` made | same |
| The Extensions page in a browser (built-in pane, real install) | `Runs on: legacy-env`, each row `Requires: gradio-host (legacy-env)` or `uv (legacy-env)` | not run (no browser on the WSL side) |
| Grep guard | `go test ./internal/platform -run MainEnv` failed on 7 comments and 1 message before they were reworded; passes now | not run on Linux (the test only reads files) |

`gofmt`, `go vet ./...` and `go test ./...` pass in `core` (Windows).

### Only in tests, not run for real

- `requires` between two catalog entries (install order, removal refused): fake catalog and a fake program
  (`extcatalog/requires_test.go`). No real catalog entry has an explicit `requires` yet.
- `--system-python` (the `system` platform): `Command` and `Status` are unit-tested; no run with the system Python,
  Docker or Colab.
- `TestRealPipInstallThroughBridge` (needs a throwaway venv and a tiny entry), macOS, `-race` (cgo), the Docker image.
- A real install of an in-process tab (pip into the main env) through the new `Create`: the code is the old command
  line, checked by tests that read the command that would run.
- The Vite pages other than the launcher's Extensions page. The embedded bundle was rebuilt with Node 22.9 from the
  existing env, which Vite warns about (it asks for 22.12); `tsc -b` and the build ran.
