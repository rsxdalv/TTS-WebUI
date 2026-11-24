# platform/uv: the uv platform (P2, in progress)

Design: D23, D25, D28, D29 in [PLAN.md](../../../../documentation/core-rewrite/PLAN.md); layout in
[PLATFORM-DESIGN.md](../../../../documentation/core-rewrite/PLATFORM-DESIGN.md) section 5. This file says what was
built, what differs, and what was run. P2 is not finished: see "Not done".

## What it is

| Piece | Where |
|---|---|
| `platform/uv` | `Install` (pinned uv 0.13.6 with the checksums of the release's `.sha256` files, Python 3.10.11 by uv into `platforms/uv/python`, cache folder, same-volume hardlink test, `platform.json` written last); `Create` / `Remove` / `EntryStatus` for an own-venv entry (`.venvs/<package>`, `entry.json` last, `installing.json` while unfinished); `Command` (`uv`, or the venv's python run directly). |
| `torchrows` | The one torch table: rows `cu128`, `cpu`, `rocm7.2`, `xpu`, `pypi`; default row per GPU choice and OS; GPU detection (`nvidia-smi`, `rocm-smi`, Apple silicon). The legacy installer reads its version constants from it. |
| `extcatalog` | `Options.UV`; plan = platforms first, then entries; `409 {needs: gpu_choice}` until the install carries `gpu`; `GET /api/extcatalog/uv-platform`; manifest fields `python` and `torch` read. |
| `requires` | `Graph.Installable`, `Plan.Platforms`: a platform that can be installed is a step, not a refusal. |
| `core/ui` Extensions page | GPU panel on the 409, Cancel button. Embedded bundle rebuilt. |

## Differences from the plan and the design

- `uv` is no longer provided by `legacy-env` or `system`. An engine on a legacy install is told "install the uv platform"
  (it used the main env's uv before). Existing venvs without `entry.json` (older installs) are still "installed".
- xformers is installed on the NVIDIA row only. The old native install installed it on every row; 0.0.35 has no macOS wheel.
- A venv whose last install did not finish is deleted and started again (found in the real cancel run: uv will not
  install over a half-linked package). Downloads are in the cache, so nothing is fetched twice.
- Removing an engine frees little disk: its files are hardlinks into `cache/`, which keeps them for the next engine.
  The page says both numbers (venv size; free space before and after) and the cache size. Clearing the cache is not built.
- The Python for a venv is chosen with `--python <version> --managed-python --no-python-downloads` (the env var
  `UV_PYTHON_PREFERENCE` conflicts with `--managed-python` and was dropped). `UV_NO_CONFIG=1` keeps uv away from
  `pyproject.toml`.

## Verification (2026-11-24)

| Line | Windows | Linux (WSL) |
|---|---|---|
| Unit tests (fake uv = the test binary run as `uv`; stand-in through `Options.Prefix` in extcatalog) | pass | not run |
| `TestRealInstall` (`UV_REAL_ROOT`): real uv and Python download, venv, `pip install six` | pass | not run |
| Launcher in a fresh folder (no `installer_files`), `POST /api/ext/op/install` an engine without `gpu` | 409, nothing made | 409 `needs: gpu_choice`, proposed NVIDIA (GTX 1070 seen through `nvidia-smi` in WSL), nothing made |
| Same with `gpu: CPU`: uv platform then the engine (cold cache) | completed (Piper) | completed (Kitten, then Piper) |
| Cancel while the engine downloads, install again | Piper while linking and Kitten while downloading: both completed on the second run | Kitten (client dropped during resolve/download): listed "not installed", the install again removed the unfinished venv and completed |
| A torch file in two venvs is one file | `torch_cpu.dll`: `fsutil hardlink list` shows both venvs and `cache/archive-v0` | `libtorch_cpu.so`: inode 951440 in Kitten and Piper, 3 links with the cache (2 after Piper was removed) |
| Remove one engine; the other still works | Piper removed (31550 files, 1.3 GB, disk freed 11.4 MB); Kitten still imports torch | Piper removed while running (33453 files, 1.8 GB, disk freed 43.3 MB); Kitten generated a clip afterwards |
| Generate one clip from the Vite engine page (no Gradio host: "gradio not started") | Kitten clip (3 s) clicked in the real browser pane; a second clip (4 s) after Piper was removed while running | clips (214844 and 207644 bytes) made with `gradio_client` on the page the iframe shows (`http://127.0.0.1:7770/<pkg>/`); **no browser on WSL, nothing was clicked** |
| Extensions page in a browser, fresh folder: Install on Kitten asks the GPU, Cancel during the download | GPU panel, install log and Cancel seen in the browser pane. The install used the preselected NVIDIA row (a radio click missed the screen); it was cancelled and a CPU install was not finished in the page | not run |
| Removing a running engine | found a bug: Windows refused to delete the running `python.exe` and left a half-deleted venv. Fixed (the running process is stopped first); the re-run passed | passed in the same flow |
| Install pre-flight (Windows path length, free disk) | unit tests only; not run against a real deep folder or a full disk | not run |

## Not done

- Tools provider (git, ffmpeg on the engine's PATH, D26). Today `git+https` requirements use the machine's git.
- uv platform uninstall; clearing the wheel cache.
- Venv folder name: measured the deepest file of a real torch venv at 138 characters below the venv folder, so the
  pre-flight allows 150 and refuses a Windows install folder that would pass 259. The folder name was not shortened.
- `--system-python` with the uv platform, Docker, macOS, the NVIDIA row with a real GPU (only the CPU row was installed).
- The engine page was not opened in a browser on Linux. The iframe points at the Gradio port with the page's own
  protocol, so a TLS public port (`ssl_*` in config.json) is not handled.
- Found, not fixed (P4): on a fresh Linux folder the launcher logs `could not record the app processes ...
  installer_files/app-processes.json: no such file or directory`.
