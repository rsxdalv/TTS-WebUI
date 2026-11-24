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

