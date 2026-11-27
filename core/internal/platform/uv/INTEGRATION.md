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
| `platform/uv` tools | The tools provider (D26): `tools.go`, `toolspin.go`. See "Tools provider". |
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

## Tools provider (D26)

What each tool is, per OS (decided from what was reachable and what a real engine needed, 2026-11-24):

| Tool | Windows | Linux | macOS |
|---|---|---|---|
| git | MinGit 2.56.1 zip (amd64, arm64), checksum from the release asset digest; `cmd/` on PATH | dugite-native 2.56.1 `.tar.gz` (amd64, arm64), **only when the machine has no git**; `bin/` on PATH, `GIT_EXEC_PATH`/`GIT_TEMPLATE_DIR` set (LAB-47, below) | the system's git (`xcode-select --install`); refused before any download when missing |
| ffmpeg | BtbN FFmpeg-Builds `gpl-shared` 8.1.3 zip (amd64); `bin/` on PATH | same build as `.tar.xz` (amd64, arm64); `bin/` on PATH and `lib/` on `LD_LIBRARY_PATH` | none pinned (BtbN has no macOS builds): the system's (`brew install ffmpeg`) |

Why the **shared** build: the program alone is not enough. Every engine venv gets `torchcodec`, which loads
ffmpeg's shared libraries (`libtorchcodec_core4`) and fails with "Could not load this library" when there are none
(seen on Windows and Linux in a real Kitten venv before the tools existed). A static `ffmpeg` would pass `ffmpeg -version`
and still fail there. torchcodec loaded and decoded with the pinned 8.1.3 on both OS. The build is the BtbN **monthly**
autobuild of 2026-10-31 (monthly ones are kept; daily ones are removed). The pins are in `toolspin.go`; changing one is
a reviewed change.

Where: `platforms/uv/tools/<name>/`, extracted beside it as `<name>.new` and swapped, `tool.json` written last. A `.tar.xz`
is unpacked with the platform's own Python (`tarfile`; Go has no xz in its standard library; symlinks are kept, a path
that leaves the folder is refused). `Install` fetches the tools; `Create` fetches what is missing, so a platform
installed before this step upgrades itself on the next engine install. They are first on PATH in `Platform.environ`,
which every command is built from: `uv` (a `git+https` requirement runs git) and an own-venv engine (extprocs takes the
platform command's environment).

### Linux git (LAB-47, 2026-11-26)

**Decision (proposal, for the human to confirm): a pinned git for Linux amd64 and arm64 (dugite-native v2.56.1), fetched only
when the machine has no git; where there is no pin (macOS, other architectures) the system's git, refused before the first
download.** The first version of this section said no source was trustworthy; that was wrong, dugite-native (the git GitHub
Desktop embeds) was missed and was found on review.

The failure, reproduced with the real launcher on Ubuntu: `PATH` an empty folder, a fresh root under `/tmp`,
`POST /api/ext/op/install {"package_name":"tts_webui_extension.kitten_tts","gpu":"CPU"}`. uv (16 MB), Python 3.10.11 and ffmpeg
(68 MB) were downloaded, then step 2 failed with "installs from a git URL and git is not on this machine". Now the same request, with
a PATH of basic programs (`sh sed grep ...`) and no git, downloads git 2.56.1 (64 MB), checks it, unpacks it (152 MB; the x64 build
carries Git Credential Manager, 80 MB), runs it, and installs Kitten TTS (uv) from its `git+https` URL to `{"done":true,"ok":true}`.

The source, read on 2026-11-26 (`desktop/dugite-native`, tag `v2.56.1`, commit 238326e of 2026-11-10; git 2.56.1, the same
version as the Windows MinGit pin):

| | |
|---|---|
| Maker | the GitHub Desktop team; built by the repo's own CI (`script/build-ubuntu.sh`), assets uploaded by `release.yml` |
| Assets | `dugite-native-v2.56.1-238326e-ubuntu-x64.tar.gz` (64 MB), `...-ubuntu-arm64.tar.gz` (24 MB). No top folder; members start with `./` |
| Checksums | a `.sha256` file beside each asset; both matched a real download: x64 `e5bc3e8e...111d`, arm64 `8626f036...abcb6`. They sit next to the file, so they catch corruption, not a replaced release: the pin is the defence, as for every tool here |
| Licence | the repo is GPL-2.0 (git's); git-lfs and Git Credential Manager inside are MIT; `libexec/git-core/NOTICE` lists them |
| Not signed | no signature on the assets or the checksums. The same trust as ffmpeg (BtbN) and MinGit: a pin of a hash read once |
| **Not self-contained, despite the README ("no linking to system libraries")** | `readelf`: `bin/git` needs `libz`, `libc` (GLIBC_2.34); `git-remote-https` also needs the **host's `libcurl.so.4`** (and its dependencies). Built on Ubuntu 22.04. So: glibc 2.34 or newer (Ubuntu 22.04+, Debian 12+, RHEL 9+), libcurl installed |
| Relocatable | only with `GIT_EXEC_PATH` and `GIT_TEMPLATE_DIR` (the prefix is compiled in as `/`; without them: "unable to find remote helper for 'https'"). Both are set in `toolEnv` for every command of the platform. System config (`/etc/gitconfig`) still applies |
| arm64 | not run (no arm64 machine here); readelf shows the same needs, checksum matches a real download |

Because it links the host's libraries, **the pin is a fallback, not a replacement**: `OnlyIfMissing` fetches it only when git is not
on the PATH the install would use, so a machine whose git works never gets one that may not. After unpacking, before the folder is
swapped in, `Smoke` runs `bin/git --version` and `git-remote-https` from the new folder; a program that cannot load (missing
libcurl, old glibc) fails the install with the program's own words and "install git with your package manager instead", and the
unpacked folder is removed. A wrong checksum is refused before unpacking (the same `download` as the other tools).

What else was built: the pre-flight for entries with a `git+` requirement (`Platform.CheckGit`, `extcatalog.checkGitForPlan`, also run by
`Create`): `409 {"needs":"git"}` before anything is downloaded, with the per-distro commands, for macOS and for a Linux
architecture with no pin. A git file on PATH must be executable. Docker: the image installs `git`. Colab: the notebook starts
with `!git clone`.

Other sources looked at, and why not:

| Source | Verdict |
|---|---|
| git-scm.com / kernel.org | source only |
| Git for Windows MinGit | Windows only (used there) |
| Debian/Ubuntu `.deb` | not portable across distros (Ubuntu 24.04 git needs glibc >= 2.38, libcurl3t64-gnutls, pcre2, expat, perl; exec path compiled in) |
| conda-forge `git` 2.56.1 | needs conda's solver, prefix rewriting and 8 more packages (libcurl, openssl, libexpat, libiconv, libzlib, pcre2, libgcc, perl); two builds of one version differ in checksum |
| other "static git" repos on GitHub (`darkvertex/static-git`, `EXALAB/git-static`) | not inspected: one maintainer each; dugite-native supersedes them |
| Homebrew, Nix, Alpine | not researched |

Not built, not run: rewriting `git+https://github.com/<o>/<r>@<ref>` to GitHub's archive URL so uv needs no git (it would not cover
transitive `git+` requirements, builds that read `.git`, other hosts). macOS has no pinned git (dugite-native has macOS builds;
not looked at). Found, not fixed: `pythonExe` takes the first output line of `uv python find`, so uv's deprecation warning when
`UV_NATIVE_TLS` is set (as in this container) is taken as the Python path and a `.tar.xz` unpack (ffmpeg) fails; the real runs here
unset that variable.

## Install steps and parallel fetches (LAB-58, 2026-11-27)

An install is a text stream, and a long uv step (torch is 2.4 GB) printed nothing for ten minutes, because uv draws
progress bars only on a terminal. Now the stream carries the steps too.

- `platform.Reporter` (`platform/events.go`) travels in the context: `Plan` announces the steps, `Step` changes one
  (state, bytes or packages done/total, a detail line). With no Reporter every call does nothing.
- The install answer of `/api/ext/op/install` is NDJSON: `{"line"}` as before, plus `{"plan": [{id, label}]}` first and
  `{"step": {id, state, done, total, unit, estimated, detail}}` as steps change. A client that only reads `line` and
  `done` is not affected. Ids: `uv`, `python`, `git`, `ffmpeg` for the platform; `<package>:venv`, `:torch`, `:wheels`,
  `:requirements` for a venv; the package name for any other install.
- Progress: the platform's own downloads report bytes. uv's torch step has no numbers: the cache folder's growth is
  measured (the folders with a top-level `torch/`, in `archive-v0/` and in the `.tmpXXXX` folder uv unpacks into),
  against a guess of the unpacked size (`torchUnpackedBytes`: the row's `ApproxMB` times 4/3), so the bar says "about".
  uv writes its biggest files (torch_cuda.dll, 1 GB) in one go at the end, so the bar moves in jumps. The requirements
  step counts uv's own `Downloading` / `Downloaded` lines (packages).
- The Extensions page shows them in a dialog (`InstallProgress`); `core/ui/mock-install.html` plays a made-up install
  for looking at it with `npx vite` (no launcher needed).

Parallel (`parallel.go`, `prefetch.go`):

- `Install` runs the Python install and the tool downloads (git, ffmpeg) at the same time; the tools also run beside
  each other. The first error cancels the rest. `emit` is wrapped (`lockedEmit`) because callers keep a log slice.
- `Create` fetches the requirements' wheels into the cache while torch downloads (`uv pip compile`, the torch family cut
  out of the pins, `uv pip install --no-deps --target <scratch>`), and the requirements step then finds them there.
  Failure is a log line, never a failed install. `TTS_PREFETCH_WHEELS=0` turns it off; a stand-in uv (tests) does not
  prefetch unless `Options.Prefetch`.

Measured on the Windows 10 machine (CPU row, then the cu126 row), real downloads:

| | before (serial) | now |
|---|---|---|
| step 1: Python + git + ffmpeg (CPU row run) | 46 s | 52 s (CPU row), 38 s (cu126 run) |
| requirements step | 1 m 54 s | 25 s (prefetched, 157 wheels in 2 m 18 s) |
| torch, CPU row (530 MB) | 1 m 07 s | 2 m 34 s, while the prefetch ran |
| whole install, CPU row | 3 m 51 s | 3 m 56 s |

The CPU row gains nothing: the downloads share the link, so what the prefetch saves in the requirements step it costs
torch. For the cu126 row the whole install took 15 m 57 s (torch 14 m 46 s, the prefetch done after 2 m 32 s); the
serial install earlier the same day took torch 9 m 22 s, but the link was slower in this run (the rate after the
prefetch had ended was 4.5 MB/s against 7.3 MB/s), so **no A/B was run at the same time and the gain of the prefetch on a
big torch is not shown**. `TTS_PREFETCH_WHEELS=0` is there to compare.

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
| Extensions page in a browser, fresh folder: Install on Kitten asks the GPU, Cancel during the download | GPU panel, install log and Cancel seen in the browser pane. The install used the preselected NVIDIA row; it was cancelled (the CPU install through the page was finished later: next row) | see the next row |
| Removing a running engine | found a bug: Windows refused to delete the running `python.exe` and left a half-deleted venv. Fixed (the running process is stopped first); the re-run passed | passed in the same flow |
| Install pre-flight (Windows path length, free disk) | unit tests only; not run against a real deep folder or a full disk | not run |
| Fresh folder, **launcher page only**: Install on Kitten, GPU panel, **CPU chosen in the select**, install uv + tools + engine (cold cache), open the engine, generate a clip. Started with `--start`: the Gradio host is absent | pass. Everything clicked in the browser pane (Install, the CPU option, Install uv and Kitten, the engine link, Process); `platform.json` says `gpu: CPU`, tools `git` and `ffmpeg` installed by the page's install, a 5 s clip with a waveform in the engine page | install clicked in the browser pane from the Windows side (WSL's port 17971 is reachable from Windows; platform.json `gpu: CPU`, tools: `ffmpeg` only, git is the system's). **The engine page on port 3000 is not reachable from Windows (WSL does not forward it): the engine was opened with `POST /api/go-data/engines/{pkg}/open` and the clip (275 KB wav) came from `gradio_client`; nothing was clicked there** |
| Tools provider, unit tests (fake archives over TLS: checksum mismatch leaves nothing, archive without the probe file, zip slip, PATH and `LD_LIBRARY_PATH` order, `git+` with no git is refused before a venv starts) | pass | pass (the whole package, Linux test binary) |
| `TestRealTools` (`UV_REAL_ROOT`, `UV_REAL_ENGINE_PYTHON`): install with the tools into a fresh folder; with PATH limited to the Windows system folders, uv installs `git+https://github.com/pypa/sampleproject.git`; the venv's python finds the pinned ffmpeg; the Kitten venv's torchcodec decodes a wav | pass (git from `tools/git`, ffmpeg 8.1.3) | pass (ffmpeg from the tar.xz; **git was the system's**, so git was not tested as removed) |
| Negative control: the same torchcodec decode without the platform's tools | fails (Could not load libtorchcodec_core4.dll) | fails (libtorchcodec_core4.so) |

## Checksum refusals

A wrong checksum of uv, ffmpeg and git with real downloads, a 404, a cut-off and a truncated download, each with a retry, and the pin of every archive against its server: [CHECKSUM-CHECKS.md](../../../../documentation/core-rewrite/CHECKSUM-CHECKS.md) (Linux, 2026-11-26). It found that `uv python find` warnings broke the ffmpeg unpack; fixed.

## Not done

- Tools: not run on macOS; the Linux git pin was run on Ubuntu x64 only (real download, smoke test, a `git+https` install through `TestRealTools` and the Kitten install through the launcher API, with no git on PATH); **not run on arm64, on a host without libcurl or with glibc older than 2.34** (those are covered by fake archives whose programs fail to load), and not through the launcher page in a browser; an engine that calls the `ffmpeg` program and not only torchcodec was not run (`shutil.which` and `ffmpeg -version` through the venv's python were).
- The GPU choice is fixed at platform install and the page preselects the proposed GPU (LAB-46, **documented, deferred by the human on 2026-11-24**). For later: (1) a detected GPU should be a suggestion, not a preselection one click accepts; (2) the choice must be changeable (upgradable) after the install, which means a second torch row per entry and the extra download shown. Nothing was built for either.
- uv platform uninstall; clearing the wheel cache.
- Venv folder name: measured the deepest file of a real torch venv at 138 characters below the venv folder, so the
  pre-flight allows 150 and refuses a Windows install folder that would pass 259. The folder name was not shortened.
- `--system-python` with the uv platform, Docker, macOS, the NVIDIA row with a real GPU (only the CPU row was installed).
- The engine page was not opened in a browser on Linux. The iframe points at the Gradio port with the page's own
  protocol, so a TLS public port (`ssl_*` in config.json) is not handled.
- Found, not fixed (P4): on a fresh Linux folder the launcher logs `could not record the app processes ...
  installer_files/app-processes.json: no such file or directory`.
