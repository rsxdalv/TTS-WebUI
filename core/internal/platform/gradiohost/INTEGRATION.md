# platform/gradiohost: the Gradio host as an entry (P3, in progress)

Decisions D23, D24, D29 of [PLAN.md](../../../../documentation/core-rewrite/PLAN.md); item P3. This file says what was
built, what differs from the plan, and what was run. **P3 is not finished: see "Not done".**

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

## Verification

Unit tests only so far (Windows, Go 1.27.4): `gradiohost` (fake uv: steps, constraints file, partial then complete,
removal), `detect` (new folder, legacy appearing later, system wins), `extcatalog` `host_test.go` (plan order, install
through the NDJSON handler, removal refused with a tab and while running, no tab installed without a host, adopted env
refuses), launcher (`/` of the public port with no host is a page, not a 502), supervisor (browser opens the Vite UI when
Gradio is not coming). `gofmt`, `go vet ./...` and `go test ./...` pass in `core`. The launcher UI bundle was rebuilt
with Node 22.9 (`tsc -b`, `vite build`); the page was not opened in a browser.

## Not done

- **No real run of anything.** The host was never installed for real (uv, torch, `requirements.txt`, then Gradio up behind
  the router), no tab was installed through it, and nothing was removed for real. None of the four demo lines ran.
- The Vite menu: every generation page still shows; there is no "backend installed" state and no install link yet.
- Pip list per venv: works because pip is in the host venv; not run for real.
- `diagnostic.go` (one env), `procscan_other.go` (word `micromamba`), npm for the launcher's React build on the host
  platform (`Command("npm")` is refused there: P4 owns the build; `react-ui/out` must be shipped).
- Removing the host while the app runs is refused by `AppRunning`; the launcher passes `l.sup.Running`. Not run.
