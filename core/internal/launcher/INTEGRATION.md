# launcher: a default install without micromamba (P4)

Item P4 of [PLAN.md](../../../documentation/core-rewrite/PLAN.md); decisions D26, D30, D32. This says what was
built, how it differs from the plan, and what was run. What was only in tests or not run is at the end.

## What it is

| Piece | What |
|---|---|
| Steps by folder | `GET /api/steps/status` answers `download, preflight, update` for a new folder. `mamba, gpu, install` (and their markers) exist only for a folder that has `installer_files` or one of `.gpu .git_version .python_version .pip_packages` (`legacyInstall`, `steps.go`); on any other folder those three POSTs answer 400 and create nothing. `--system-python` is never legacy. |
| Update of an archive install (`archive.go`) | `archive.json` (`{commit, built}`) is at the archive's root and CI publishes the same file beside the zip as `tts-webui-installer.json`. Status compares the two (one small GET, cached for a minute). Update downloads the zip (`TTS_ARCHIVE_URL` overrides the release URL, for a mirror or a test) and extracts it over the install. Refused with the app running (409). The step's stream ends with exit code 1 when it failed (it used to end 0 whatever happened; the download step too). |
| Running binary (`selfswap.go`) | The zip entry that is the running launcher is written as `<exe>.new`. At the next start `SwapPendingBinary` renames the exe to `<exe>.old`, moves `.new` in, runs it with the same arguments and exits with its code. A failed swap is reported on stderr and the old launcher keeps running. |
| Cannot check | A folder with no `archive.json` and no `.git` says its version is unknown; a git clone says to use `git pull`; no network or a 404 says "Could not check for an update: ...". Each is `detail` on the update step, shown amber in the Setup tab; none is "up to date". An old-env install with no git on PATH says so too (it used to count as up to date). |
| Setup tab | The Start button no longer waits for the update step (an update that is waiting or cannot be checked never hides it). `detail` is shown under a step. |
| Notice | `GET /api/app/notice` is `{legacy, text, url}`; a legacy folder gets the text and the link to [MIGRATION.md](../../../documentation/core-rewrite/MIGRATION.md) on both launcher pages. The Vite UI does not show it. |
| React UI | `react-ui/out` is shipped. A folder without it says to run Update or download the zip again. A clone that builds it itself runs `npm` from PATH (`react_ui_mode.go`); only the old env and `system` run it through the platform. |
| ffprobe | `react_ui_backend` also looks in `platforms/uv/tools/ffmpeg/bin` (Linux: `LD_LIBRARY_PATH` set to the tool's `lib`). The FFMPEG Metadata page, on the 503, links to the launcher's Extensions page (installing any engine installs uv first, which fetches ffmpeg). |
| db | The `db` page, `databaseApi.ts` and `VITE_DB_API_URL` are deleted from `react-ui`. The old installer step still sets up its database, for an old-env install. |
| Docker, Colab | The image has uv 0.13.6 (the pin) on PATH from pip, builds `react-ui/out` in a Node stage and has no Node, and copies the commit being built instead of cloning `main`; `.dockerignore` keeps a developer's state out. The Colab notebook runs `pip install uv==0.13.6`. Both run on `--system-python`. |
| CI | `update-full-repo-zip.yaml` builds `react-ui/out`, writes `archive.json` and publishes it beside the zip. `test-pip-install.yaml` now builds the launcher and runs `core/scripts/ci/install-host.sh` (the install of `gradio-host` through the launcher) on Linux, macOS and Windows. |

## Differences from the plan

- **"Update replaces the binary, the UI and the sources together"** is done by extracting the archive over the install,
  not by a staged swap of folders: an update that is interrupted can leave old and new files mixed until it is run
  again. It overwrites tracked files, so local edits to the sources are lost; user data is not in the archive.
- **The version check is `archive.json`, not git.** The plan did not say how; a zip made by hand has none and is told so.
- **"The ffmpeg-metadata page offers the tools install"** is a link to the Extensions page, not a button: the uv
  platform cannot be installed on its own from that page, only as the first step of an entry.
- **The database set-up is not removed from the old installer step** (old-env installs keep it); the new install has none.

## Verification

Run on 2026-11-25 from `platform/p4-default-install`, Windows 10 (Go 1.27.4) and WSL `tts-linux-test`, in scratch folders
under the OS temp directory. A "fresh folder" is `git clone` of the branch (so it has `.git`, as the zip does) plus
`react-ui/out` built by `npm run build:static`, a launcher built from the branch and an `archive.json` written by hand
(what CI now does). The zip packaging by CI itself was not run. The launcher was started with PATH holding only the
Windows system folders (Windows) or an empty folder (WSL); `python`, `git`, `node`, `npm`, `uv`, `micromamba`, `ffprobe`
were all reported absent first.

| Line | Windows | Linux (WSL) |
|---|---|---|
| Fresh folder, no Python/git/Node: launches (`--no-browser --start`), Go pages work | steps are `download, preflight, update`; `:3000/` and `/history` 200, `host-status` `installed:false, installable:true`, `:7770/` answers the no-host page; Setup tab in the browser pane: download and pre-flight done, update amber with "Could not check for an update: ... HTTP 404" (the release has no json yet), Launch button shown | same by curl: steps, `:3000/`, `/history`, `host-status`, `:7770/` all as on Windows; no browser |
| Install uv and an engine, generate | `tts_webui_extension.kitten_tts` through the launcher API with the GPU choice `CPU` (cold cache): uv, Python, tools (`git` came from `platforms/uv/tools`, the install used `git+https` with no git on PATH), engine venv; opened from `/engines` in the browser pane, Input typed, Process clicked: a 7 s clip with a waveform | same engine through the launcher API (`CPU`). **With git absent from PATH it failed, by design: on Linux no git is fetched (`git: not fetched on Linux ... apt install git`), and Kitten installs from a git URL.** With git and coreutils on PATH (python, node, uv absent) it installed (cold); the engine opened with `POST /api/go-data/engines/{pkg}/open`, and a 190844-byte wav came from `gradio_client` on the page the iframe shows. **Nothing was clicked** |
| Update of an archive install | a local file server stood in for the release (`TTS_ARCHIVE_URL`): status said an update is waiting; POST update downloaded 10 MB, extracted 171 entries, saved `tts-webui.exe.new`, exit code 0 on the stream; after restarting the launcher `tts-webui.exe` is the new build (`--version` changed), `.old` is there, `.new` is gone, the launcher answers | not run (the swap and the extraction are unit-tested on Linux: `TestSwapPendingBinaryReplacesAndRuns`, `TestExtractWritesTheRunningBinaryBesideItself`) |
| Old install updated to this version | scratch root with the real env through a junction and the real markers: six steps, `legacy: true` on `/api/app/notice`, `Runs on legacy-env` ready, 67 entries, Gradio on 7770 answered `/config` (1.1 MB) and the React front came up; the launcher page (a folder with the `.gpu` marker only) shows the amber notice with the guide link above all six steps | six steps (markers copied from a P3 scratch: `.gitignore` only), `legacy: true`, Gradio `/config` 1.1 MB, React ready, the real env untouched (symlink removed afterwards) |
| `install-host.sh` (the CI job) | ran to the end: gradio-host installed (gradio 5.49.1, torch 2.11.0+cpu, `import tts_webui` works) | not run |
| Docker image | not run: no Docker daemon was running on this machine | not run |

`gofmt`, `go vet ./...` and `go test ./...` pass in `core` (Windows). The launcher package's whole test suite also passes on Linux
(a cross-built test binary run in WSL from the package folder).

### Only in tests, not run for real

- The Docker image build and start (no Docker daemon was running). The Dockerfile change is reviewed, not built.
- The CI workflows (`update-full-repo-zip.yaml`, `test-pip-install.yaml`): written, YAML not run on GitHub. `install-host.sh` ran on
  Windows only; the zip, `archive.json` publishing and `tts-webui-installer.json` are untested until the workflow runs on `main`.
- The real release has no `tts-webui-installer.json` yet, so the real update check answers "Could not check (HTTP 404)" until CI publishes it.
- Update on Linux and macOS (unit tests only); a swap while the launcher runs under a supervisor script; macOS.
- The Colab notebook change (`pip install uv==0.13.6`).
- Windows path length of `.venvs` in a deep folder; an update interrupted half way (files mixed until it is run again).
