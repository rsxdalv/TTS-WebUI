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
| git | MinGit 2.56.1 zip (amd64, arm64), checksum from the release asset digest; `cmd/` on PATH | **the system's git** (decision below, LAB-47). A missing git is refused before the first download of the plan | the system's git (`xcode-select --install`) |
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

**Decision (proposal, for the human to confirm): the system's git, checked before anything is downloaded. No pinned Linux
git.** Rob had not chosen; this is the lower-risk of the two and is built. The other is described so it can be reversed.

The failure, reproduced with the real launcher on Ubuntu: `PATH` an empty folder, a fresh root under `/tmp`,
`POST /api/ext/op/install {"package_name":"tts_webui_extension.kitten_tts","gpu":"CPU"}`. Result: uv (16 MB), Python 3.10.11,
**ffmpeg (68 MB)** were downloaded, then step 2 failed with "installs from a git URL and git is not on this machine". The check
already existed in `Create`, but after `installTools`, and the message ran into the "not fetched on Linux" hint. After the
change the same request is answered at once with `409 {"needs": "git", "error": "Kitten TTS (uv) installs from a git URL and git
is not on this machine. Install it with your package manager: apt install git (Debian, Ubuntu), dnf install git (Fedora), ...
Then install again (nothing was installed)"}` and the root holds only `extensions.json`. With git on PATH the same request goes on to
the GPU question as before.

What was built: `Platform.CheckGit` (also used by `Create`, before the tools are fetched, as the backstop for pasted entries);
`extcatalog.checkGitForPlan` on the plan, before the GPU question, for every own-venv entry of the plan with a `git+`
requirement; a git file on PATH must be executable. Where the platform fetches git (Windows) a missing git is not a refusal.
Docker: the image installs `git` (Dockerfile, `apt-get install ... git`). Colab: the notebook starts with `!git clone`, so git is
there.

Is there a git binary the platform could pin for Linux (amd64, arm64) that is checksummable, relocatable and trustworthy?
Looked at on 2026-11-26; **none that I would trust**:

| Source | Licence | Checksum | Relocatable, no root | Verdict |
|---|---|---|---|---|
| git-scm.com / kernel.org | GPL-2.0 | tarballs signed (source) | source only, no binaries | not a binary |
| Git for Windows MinGit | GPL-2.0 | yes (used on Windows) | Windows only | not Linux |
| Distro packages (Debian/Ubuntu `.deb`) | GPL-2.0 | yes (Packages index) | no. The Ubuntu 24.04 `git` here needs glibc >= 2.38, `libcurl3t64-gnutls`, `libpcre2-8`, `libexpat`, `libz`, perl; its exec path is compiled in (`/usr/lib/git-core`; 166 helpers, 11 MB) | not portable across distros; extracting a `.deb` needs `ar` + zstd |
| conda-forge `git` 2.56.1 (`linux-64`, `linux-aarch64`) | GPL-2.0-or-later and LGPL-2.1-or-later | sha256 on api.anaconda.org | needs conda's prefix rewriting and 8 more packages (libcurl, openssl, libexpat, libiconv, libzlib, pcre2, libgcc, perl); two builds of the same version (`_0`, `_1`) have different checksums | would need micromamba as a second package manager; not one archive |
| Third-party "static git" repos on GitHub (found: `darkvertex/static-git`, `EXALAB/git-static`) | not checked | not checked | linux-64 only as far as the search page says | **not inspected** (GitHub access of this session is limited to this repository) and unsigned builds from a single maintainer: the platform would run them for every `git+` install. Not trusted |
| Homebrew, Nix, Alpine builds | | | | not researched: each needs its package manager or a musl userland, and none turned up in the search as a standalone archive |

So adding a Linux row to `gitTools` would mean pinning a build from an individual, or shipping a second package manager. A pin
here is run on every `git+` install, and "changing one is a reviewed change": that review has nothing upstream to rest on.
If a trustworthy build appears (an upstream or distro project publishing signed static `git` + `git-remote-https` for both
architectures), the work is one `gitTools["linux/<arch>"]` row, `Path`, `Probe` and `GIT_EXEC_PATH` (and `GIT_TEMPLATE_DIR`) set in
`toolEnv` next to `LD_LIBRARY_PATH`; the checksum, wrong-checksum and nothing-partial paths are the ones ffmpeg already uses.

Not built, not run: rewriting `git+https://github.com/<o>/<r>@<ref>` to GitHub's archive URL so uv needs no git at all. It would
not cover transitive `git+` requirements, builds that read `.git` (setuptools-scm), or other hosts, and `@main` would change what
is installed. Left as an idea.

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

## Not done

- Tools: not run on macOS; the missing-git refusal was run through the real launcher on Ubuntu (see Linux git) but a complete engine install on a machine that then gets git was not repeated (the earlier Linux runs used the system git); an engine that calls the `ffmpeg` program and not only torchcodec was not run (`shutil.which` and `ffmpeg -version` through the venv's python were); the tools install was not run through the launcher page (only through `Platform.Install`).
- The GPU choice is fixed at platform install and the page preselects the proposed GPU (LAB-46, **documented, deferred by the human on 2026-11-24**). For later: (1) a detected GPU should be a suggestion, not a preselection one click accepts; (2) the choice must be changeable (upgradable) after the install, which means a second torch row per entry and the extra download shown. Nothing was built for either.
- uv platform uninstall; clearing the wheel cache.
- Venv folder name: measured the deepest file of a real torch venv at 138 characters below the venv folder, so the
  pre-flight allows 150 and refuses a Windows install folder that would pass 259. The folder name was not shortened.
- `--system-python` with the uv platform, Docker, macOS, the NVIDIA row with a real GPU (only the CPU row was installed).
- The engine page was not opened in a browser on Linux. The iframe points at the Gradio port with the page's own
  protocol, so a TLS public port (`ssl_*` in config.json) is not handled.
- Found, not fixed (P4): on a fresh Linux folder the launcher logs `could not record the app processes ...
  installer_files/app-processes.json: no such file or directory`.
