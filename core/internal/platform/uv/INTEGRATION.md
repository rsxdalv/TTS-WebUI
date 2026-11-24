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
| Launcher in a fresh folder (no `installer_files`), `POST /api/ext/op/install` Piper without `gpu` | 409, nothing made | not run |
| Same with `gpu: CPU`: uv platform then Piper (cold cache) | completed | not run |
| Cancel while linking (Piper) and while downloading (Kitten TTS), install again | both completed on the second run | not run |
| `torch_cpu.dll` in two venvs is one file | `fsutil hardlink list`: both venvs and `cache/archive-v0` | not run |
| Remove Piper; Kitten still imports torch | 31550 files, 1.3 GB, disk freed 11.4 MB; `torch 2.11.0+cpu` | not run |
| Generate one clip | **not run** (no Vite engine page, no start-on-open yet) | not run |
| Extensions page GPU panel in a browser | **not run** | not run |

## Not done

- Tools provider (git, ffmpeg on the engine's PATH, D26). Today `git+https` requirements use the machine's git.
- Vite engine page: start an installed own-venv engine on open, show it in an iframe (D27, D31).
- Venv folder name length (accepted in the design: measure on a real install, then shorten).
- Disk-space check before an install (D30); uv platform uninstall; clearing the wheel cache.
- `--system-python`: the uv platform works there in principle (it needs nothing from the main env) and was not run.
- Linux, macOS; the GPU panel; NVIDIA row with a real GPU (only the CPU row was installed).
