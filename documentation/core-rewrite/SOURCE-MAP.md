# Source map: old sources to the Go core and its Vite UI

Read-only inventory of `origin/main` at `b844e0a` (2026-11-26, P8 #756). Nothing was run: every row comes from files, git
history and the rewrite docs. "Old" is the Python/Gradio/Node world as of `2e57013` (2026-09-07, last commit before the
Go work). "New" is `core/`, the Vite `react-ui` and the platform entries.

**Relation values:** `ported` (same job, new code) · `replaced` (different design) · `wrapped` (old code still runs, new
code starts or feeds it) · `dropped` (removed or no longer used, no successor) · `not moved` (still old, nothing new).
**Confidence:** H = a doc line or a deletion in git says so; M = read from code, no doc says so; L = inferred.

## Summary

Rows in the section 3 tables (a row is a module or file group, not a file; 81 rows in total):

| ported | replaced (different design) | wrapped | dropped | not moved | new (no old row) |
|---|---|---|---|---|---|
| 26 | 22 | 15 | 2 | 14 | 2 |

By extension count (from `extensions.json`, 68 entries): 26 in-process tabs (12 `ported`/`replaced` by Go services and Vite pages, 2
`wrapped` by Go services, 12 `not moved`), 40 own-venv engines (all `wrapped`: Go starts them), 2 platform entries
(`new`). `dropped` is only `tts_webui/database` and the Next `db` page; everything else deleted was replaced.

Headline facts:

- The Python proxy tree, `server.py`, `proxy.py`, the dotenv manager and the Next.js server are gone (#744, #745).
  `server_core.py` plus `tts_webui/core_server.py` is the whole Python entry (Gradio launch only).
- **Counts in the task brief are one step behind.** `extensions.json` now has **68 entries: 26 in-process Gradio tabs, 40
  own-venv (`"proxy": "native"`) entries and 2 platform entries** (`audiocpp_pocket_tts_en`, `tts_webui_onnx.kitten_nano`).
  Before P6 it was 66 = 46 in-process + 20 native (the "46" of `MIGRATION.md`). P6 (#755) moved 20 more to own venvs.
- Of the 20 P6 engines, **3 are marked `validation: full`** (kokoro, mms, vall_e_x) and 17 `unvalidated`.
- Old installer path (micromamba, `init_app.js`) still exists and is still what `start_tts_webui.*` runs; the default
  install through the launcher no longer uses it (P4).

## 1. Timeline used for dating

| PR | Merge date | Commit | What moved |
|---|---|---|---|
| #714 | 2026-11-19 | `9b6d3b5` | Launcher + supervisor (rings 1-2), `core/ui` |
| #741 | 2026-11-19 | `6207edf` | Installer steps (`core/installer_scripts`) and react-ui backend routes in Go |
| #742, #743 | 11-20 | `dc1ab8a`, `712205a` | `server.py` port: router, extension processes, appenv, appconfig, supervise, `server_core.py` |
| #744 | 11-23 | `6a0b361` | `server.py`, `proxy.py`, `gradio_proxy_tree`, `dotenv_manager`, `extension_manager.py` deleted |
| #745 | 11-24 | `b476228` | Next.js removed, Vite app, Go serves `react-ui/out` |
| #746, #747 | 11-24 | `f3b6fb9`, `6edd548` | List tabs and leaf tabs as Go routes + Vite pages; core boots without Gradio |
| #748 | 11-25 | `1ab9437` | P0: verify on real installs (TLS in router, HF cache tab) |
| #758 (= #749, #751, #752) | 11-25 | `6eba853` | P1 platform seam, P2 uv platform, P3 gradio-host entry |
| #753 | 11-25 | `71ae227` | P4 default install without micromamba |
| #754 | 11-25 | `f41a69c` | P5 openai platform |
| #755 | 11-25 | `d2c464a` | P6 20 engines to own venvs |
| #757 | 11-25 | `4a28c22` | P7 audiocpp platform |
| #756 | 11-26 | `b844e0a` | P8 onnx platform |

## 2. Inventories

### 2.1 Old sources (what is still on disk and what it does)

| Path | Job | State now |
|---|---|---|
| `tts_webui/` | Python package: Gradio blocks, config, decorators, extension loader, utils, DB API | thin: `core_server.py`, `core_bridge/` (HTTP client of the Go control API), `core_config_bridge/`, `extensions_loader/`, `gradio/`, `decorators/`, `config/`, `utils/`, `database/`, `history_tab/`, `tutorials/`, `cli.py` |
| `server_core.py` | Entry the Go launcher runs for Gradio | replaces `server.py` |
| `extensions/builtin/` (20 dirs) | Built-in Gradio tabs (19 tabs + 1 decorator) | still shipped, 19 listed in `extensions.json` as in-process |
| `extensions.json` | Catalog: 11 groups, 68 entries, `decorators`, `example_extension`, `tabs_not_recomended` | read by Go `extcatalog` |
| `installer_scripts/` (root) | micromamba bootstrap (`init_mamba.*`), `root.*`, `init_app.js`, `js/*` (Node installer + web console, port 7771), `tools/*` (diagnostic, open_terminal), `versions.json` | still present; Go ports exist for each |
| `start_tts_webui.{sh,bat,command}` | User entry points | still call `installer_scripts/root.*` |
| `react-ui/` | Was Next.js; now a Vite SPA (50 files in `src/pages`) | rewritten in place, see 2.2 |
| `requirements.txt`, `pyproject.toml`, `Dockerfile`, `docker-compose.yml`, `installation.html` | Python deps, packaging, container, install page | Dockerfile changed in P4 (Go launcher, `--system-python`); rest unchanged |
| `tests/` (6 Python tests) | CLI, config, decorators, server integration, utils | unchanged |
| `data/ outputs-rvc/ voices/ collections/` | User data dirs | never touched by Go |

### 2.2 New sources

| Path | Job |
|---|---|
| `core/cmd/tts-webui` | The exe: `launch` (default), `--version`; applies a pending self-update swap |
| `core/internal/launcher` | Setup steps, app supervisor wiring, extension list/install API (`extensions.go`), host status, archive update, self-swap, TLS, LAN status, orphan sweep, engine limits |
| `core/internal/server`, `procs`, `paths`, `preflight`, `gpu`, `switches`, `version` | Local HTTP server (token, Host/Origin), process trees, install-root paths, system checks, GPU labels, env switches |
| `core/internal/serverpy/*` | Go port of `server.py`: `router`, `supervise`, `extprocs`, `appenv`, `appconfig`, `launchopts`, `extcatalog`, `extui`, `history`, `logview`, `piplist`, `engines` |
| `core/internal/platform/*` | Seam (`Platform`, `requires`, `detect`) and platforms: `legacyenv`, `system`, `uv`, `gradiohost`, `openai`, `audiocpp`, `onnx` |
| `core/internal/torchrows` | The one torch table (rows `cu128`, `cpu`, `rocm7.2`, `xpu`, `pypi`) |
| `core/internal/webui` + `core/ui` | Embedded launcher page (Vite, `go:embed`); source in `core/ui/src/core-app` |
| `core/react_ui_backend` | Go server for port 3000: serves `react-ui/out`, claims `/api/*` routes (18 claim names) |
| `core/installer_scripts` | Go ports of the installer scripts (`init_mamba.go`, `root.go`, `initialize_app.go`, ...) |
| `core/scripts/{smoke,ci}`, `.github/workflows/{core,build-launcher-exe,test-pip-install,update-full-repo-zip}.yaml` | Smoke tests, build and CI |
| `react-ui/` (Vite) | `routes.tsx` (react-router), `src/shims/*` (Next API shims), `src/pages`, `build-static.js` |

## 3. Map by area

### 3.1 Launcher, installer scripts and entry points

| Old | New | Relation | Evidence | Conf |
|---|---|---|---|---|
| `installer_scripts/js/server.js`, `webConsole.js`, `installerState.js`, `menu.js`, `openBrowser.js`, `index.html` (Node install console, 7771) | `core/ui` launcher page + `core/internal/launcher` + `core/internal/server` | replaced | DECISIONS D7, D12; `start_app.sh:20` has `# node js/server.js` commented out; no other caller found by grep | H |
| `installer_scripts/init_mamba.sh`, `.bat` | `core/installer_scripts/init_mamba.go` | ported | D19 (Go step default, `TTS_GO_STEPS=none` runs the script); #741 | H |
| `installer_scripts/js/initializeApp.js`, `init_app.js`, `applyDatabaseConfig.js` | `initialize_app.go`, `init_app_nodejs.go`, `apply_database_config.go` | ported | `core/installer_scripts/init_app_nodejs.go:184` still reads `versions.json`; CONTRACTS section 1 markers | H |
| `installer_scripts/root.sh`, `root.ps1` | `core/installer_scripts/root.go` | ported | `root.go:25` "init_mamba is the Go port"; the scripts remain and still call `init_mamba` then the launcher | H |
| `installer_scripts/start_app.sh`, `.bat`, `activate.*`, `init_app.*` | `start_app.go`, `activate.go`, `init_app.go` | ported | `start_app.sh/.bat` now only run `tts-webui launch --start`; `init_app.sh/.bat` and `activate.*` still run Node or activate the env | M |
| `installer_scripts/tools/diagnostic.*`, `open_terminal.*` | `diagnostic.go`, `open_terminal.go` | ported | `TTS_GO_STEPS` names `diagnostic`; `small_scripts_test.go` | H |
| `installer_scripts/tools/extension_manager.py`, `run_extension_manager.*` (port 7771) | Launcher Extensions page (`core/ui/src/core-app/ExtensionsView.tsx`) + `launcher/extensions.go` | replaced | Deleted in #744 ("extension manager ... removal") | H |
| `start_tts_webui.sh/.bat/.command` | unchanged; call `root.*`, which runs `init_mamba` and `init_app.js`, then the Go launcher | wrapped | `root.sh:7,35,50`; `start_tts_webui.sh` | M |
| Neutralino launcher (separate repo `tts-webui-ignition`) | `core/ui` ("Neutralino shell and old screens not carried over") | replaced | D7; not in this repo | M |
| `installer_scripts/enable_long_paths.reg` | referenced by `core/internal/preflight/preflight.go:178` and `root.go:124` | not moved | file still used as a hint | M |
| `Dockerfile` (cloned `main` in the image) | Same file: builds the launcher and `react-ui/out`, uv 0.13.6 on PATH, runs on `--system-python` | wrapped | `launcher/INTEGRATION.md` row "Docker, Colab" (#753) | H |
| Colab notebook | `pip install uv==0.13.6`, `--system-python` | wrapped | `launcher/INTEGRATION.md`; the notebook itself is not in this repo | L |
| `installation.html` | no counterpart found | not moved | unknown content use; not referenced by Go | L |

### 3.2 `server.py` and the Python runtime around it

| Old | New | Relation | Evidence | Conf |
|---|---|---|---|---|
| `server.py` (entry, `.env`, folders, proxy tree, React start, browser) | `server_core.py` (29 lines) + `tts_webui/core_server.py` (Gradio launch only) + Go `supervise`, `appenv`, `launchopts` | ported | Deleted in #744; `core_server.py` docstring "replaces server.py's start()" | H |
| `proxy.py`, `tts_webui/gradio_proxy_tree/*` (Python proxy tree) | `core/internal/serverpy/router` | replaced | `router/INTEGRATION.md` "replaces the Python proxy tree"; deleted in #744 | H |
| `tts_webui/dotenv_manager/*` | `serverpy/appenv` (`EnsureEnv`, `LoadEnv`, `writeenv`) | ported | `appenv/INTEGRATION.md` table; deleted in #744 | H |
| `tts_webui/config/*` (`load_config.py` defaults, `config.py`) | `serverpy/appconfig` owns `config.json`; Python reads through `core_config_bridge` | wrapped | `appconfig/INTEGRATION.md`; `server_core.py` installs the bridge first | H |
| `tts_webui/core_router_bridge`, `core_ext_bridge` (stage-3/4 shims) | `tts_webui/core_bridge/*` (13 files, HTTP client with token) | replaced | Deleted in #744; `extprocs/INTEGRATION.md` | H |
| `tts_webui/extensions_loader/extensions_data_loader.py` | `serverpy/extcatalog` | ported | Deleted between `2e57013` and HEAD; `extcatalog/INTEGRATION.md` | M |
| `tts_webui/extensions_loader/{interface,decorator}_extensions.py`, `setup_proxy_extension.py`, `proxy_harness.py`, `list_tab.py` | Python loader kept; process start/stop is `serverpy/extprocs` via `bridge.extensions`; list tabs are an iframe of Go's `extui` | wrapped | `extprocs/INTEGRATION.md`, `extui/INTEGRATION.md`; `setup_proxy_extension.py` uses `bridge.extensions.stop` | H |
| Extension subprocess spawning in Python | `serverpy/extprocs` (own-venv python run directly, limits, idle stop, max 4 running) | ported | `extprocs/INTEGRATION.md`; `launcher/engine_limits.go` (D31) | H |
| `tts_webui/history_tab/{generate_pretty_name,generate_relative_date,get_wav_files,parse_time}.py` | `serverpy/history` | ported | Deleted; `history/INTEGRATION.md` | H |
| `tts_webui/history_tab/{collections_directories_atom,delete_generation*,save_to_favorites}.py` | still Python; the tab calls `bridge.history` | wrapped | `history/INTEGRATION.md` "Python handlers are calls through core_bridge" | M |
| `tts_webui/utils/get_torch_command.py`, `pip_install.py` | `torchrows`, `platform` entry install (`Entries`), `extcatalog/install.go` | replaced | Deleted; `platform/INTEGRATION.md` | H |
| Gradio `launch()` kwargs built in Python | `serverpy/launchopts` | ported | `launchopts/INTEGRATION.md` | H |
| `launcher.Supervisor` of rings 1-2 (Go, micromamba prefix) | `serverpy/supervise` (+ `launcher/app_supervisor.go`) | replaced | `supervise/INTEGRATION.md` "the only supervisor (step 29)" | H |
| `tts_webui/gradio/{all_tabs,blocks,css,get_theme,fix_gradio_tabs}.py`, `tts_webui.css` | none | not moved | Gradio UI of the host; still what `core_server.py` builds | H |
| `tts_webui/decorators/*` (11 files) | none; applied inside host and engine processes | not moved | `TestRealExtensionProcessAppliesDecorators` (P0 baseline) shows they still run | H |
| `tts_webui/utils/*` (other 30 files) | none | not moved | not read; Python helpers for in-process tabs and decorators | L |
| `tts_webui/database/*` (FastAPI DB API) | none in Go; Vite `db` page, `databaseApi.ts`, `VITE_DB_API_URL` deleted | dropped | `launcher/INTEGRATION.md` row "db"; grep finds no importer in the repo outside docs | M |
| `tts_webui/tutorials/tab.py` | none | not moved | Gradio tab | L |
| `tts_webui/cli.py` (`tts-webui = tts_webui.cli:main` in `pyproject.toml`) | Go exe is also called `tts-webui` | not moved | name overlap; relation between the two CLIs is undocumented | L |

### 3.3 Old installer and micromamba vs the uv platform (special case b)

| Old | New | Relation | Evidence | Conf |
|---|---|---|---|---|
| `installer_files/env` (micromamba env: Python 3.10.11, git, Node, ffmpeg, every extension) | `platforms/uv/` (uv 0.13.6 pinned + sha256, Python 3.10.11 via uv, cache, `tools/{git,ffmpeg}`) and `.venvs/<entry>/` | replaced | `MIGRATION.md`; `platform/uv/INTEGRATION.md` | H |
| micromamba command prefix in `supervise`, `extcatalog`, `piplist`, `extprocs`, `react_ui_mode` | `platform/legacyenv` (only shipped code that knows `micromamba -p ... run`) | wrapped | `platform/INTEGRATION.md`; `guard_test.go` allow-list | H |
| `--system-python` flag | `platform/system` | ported | `platform/INTEGRATION.md` | H |
| Setup steps `mamba`, `gpu`, `install` (markers `.gpu`, `.git_version`, ...) | exist only for a folder with `installer_files` or a legacy marker (`legacyInstall`, `steps.go`); new folder gets `download, preflight, update` | wrapped | `launcher/INTEGRATION.md` "Steps by folder" (#753) | H |
| `.gpu` label file and GPU gate | GPU chosen when uv is installed (`platform.json`, `gpu`); gate deleted from `supervise` (D29) | replaced | `gradiohost/INTEGRATION.md` ("`StrictGPULabel` ... deleted"); `uv/INTEGRATION.md` | H |
| Torch/xformers pins in `initializeApp.js` | `core/internal/torchrows` (installer reads its constants from it) | replaced | `uv/INTEGRATION.md` table | H |
| `git pull` update | Archive update: `archive.json`, download zip, extract, `selfswap.go` | replaced | `launcher/INTEGRATION.md` (#753) | H |
| Shared env that holds the Gradio host and 46 tabs | `.venvs/gradio-host` entry (`platform/gradiohost`), optional, installed from Extensions page; tab installs via `pip` + `constraints.txt` | replaced | `gradiohost/INTEGRATION.md` (P3) | H |
| Python `uv` in main env used for native extension installs | `uv` platform; `legacy-env` no longer provides `uv` | replaced | `uv/INTEGRATION.md` "Differences" | H |
| `applyDatabaseConfig` step | kept only for old-env installs | wrapped | `launcher/INTEGRATION.md` "database set-up is not removed" | H |

What exists only for legacy-env: `platform/legacyenv`; the three steps `mamba`/`gpu`/`install` and their markers
(`steps.go`); `core/installer_scripts/{init_mamba,initialize_app,init_app_nodejs,apply_database_config,root}.go` and the
root-level scripts they port; `launcher/procscan_other.go` (orphan sweep matches the word `micromamba`);
`react_ui_backend/go_ffmpeg_metadata.go` (looks in `installer_files/env` dirs); `core/installer_scripts/diagnostic.go` ("one env", per `gradiohost/INTEGRATION.md`);
the legacy notice (`/api/app/notice`, link to `MIGRATION.md`); `TTS_GO_STEPS=none`. The grep guard
(`platform/guard_test.go`) lists the allowed files: `legacyenv/`, `installer_scripts/`, `launcher/steps.go` and step handlers,
`paths/paths.go`, `cmd_launch.go` help text, `go_ffmpeg_metadata.go`, `procscan_other.go`.

### 3.4 Extensions and tabs (special case a)

**Counts, from `extensions.json` at HEAD (computed by reading the file):**

| Kind | Before P6 (`6edd548`) | Now |
|---|---|---|
| In-process Gradio tabs (no `proxy`, no `requires`) | 46 | **26** (19 `extensions.builtin.*` + 7 third-party) |
| Own-venv (`"proxy": "native"`) | 20 | **40** |
| Platform entries (`requires`: `audiocpp`, `onnx`) | 0 | 2 |
| Total | 66 | 68 |

**Which engines moved in P6 (#755)** (the 20 whose `proxy` changed to `native`), with `validation`:

| Group | Engines | `validation` |
|---|---|---|
| TTS | `kokoro`, `mms`, `vall_e_x` | `full` |
| TTS | `chatterbox`, `styletts2`, `seamless_m4t`, `xtts_simple`, `cosyvoice`, `higgs_v2`, `vibevoice` | `unvalidated` |
| Music | `ace_step` | `unvalidated` |
| Conversion | `vocos`, `rvc`, `demucs`, `audio_separator`, `resemble_enhance`, `ap_bwe`, `pyrnnoise` | `unvalidated` |
| Conversational | `kimi_audio`, `mimo_audio` | `unvalidated` |

**The 20 older own-venv entries** (native before P6, no `validation` field): bark, parler_tts, mars5, dia, gpt_sovits,
maha_tts, openvoice, openvoice_v2, piper_tts, kitten_tts, index_tts, vox_cpm, fireredtts2, megatts3, stable_audio,
audiocraft, riffusion, audiocraft_mac, songbloom, xtts_ft_demo. They became uv-venv engines upstream before the rewrite
(e.g. #668 "convert DIA, Piper TTS and OpenVoice V2 to uv"); in the new world the Go `uv` platform builds their venv
(`Create`, `entry.json` written last) and `extprocs` runs them.

| Old | New | Relation | Evidence | Conf |
|---|---|---|---|---|
| 26 in-process tabs (list below) | still run in the Gradio host (`gradio-host` venv or legacy env) | wrapped | `extensions.json`; `requires` derived `["gradio-host"]` (PLATFORM-DESIGN section 2) | H |
| 20 pre-P6 own-venv entries | `platform/uv` `Create` + `extprocs` + router path `/<pkg>/` | wrapped | `uv/INTEGRATION.md` (real Piper, Kitten); `P0-BASELINE.md` (Piper behind router) | H |
| 20 P6 engines (table above) | same path; tab removed from the host | wrapped | `extensions.json` diff `6edd548..HEAD`; `gradio_engines.go` header; `launcher/extensions.go:93` | H |
| P6 engine API names `/api/gradio/<name>` | `react_ui_backend/gradio_engines.go` `engineOwners`: **7 names, 4 engines only** (kokoro, mms, chatterbox, vall_e_x) go to the engine's own process; every other name still goes to the Gradio host | wrapped | `gradio_engines.go:30-41`; `engine-check.ps1` `$inferenceCalls` has the same 4 | H |
| Engine env knobs lost with the shared env | manifest `env` object (`engine_env.go`, e.g. Vall-E-X torch weights setting) | replaced | `engine_env.go` comment | H |
| Unlimited concurrent engines | `TTS_ENGINE_MAX_RUNNING` (default 4), `TTS_ENGINE_IDLE_MINUTES` | replaced | `engine_limits.go` (D31) | H |

**In-process tabs (26) and what replaced or wraps them:**

| Tab (`extensions/builtin/...` unless noted) | New | Relation | Evidence | Conf |
|---|---|---|---|---|
| `extension_extension_list_tab`, `extension_extension_decorator_list_tab` | Go `extui` page in an iframe in Gradio; Vite `/tools/extensions` (read-only lists, `core-extensions`); launcher Extensions page | replaced | `extui/INTEGRATION.md`, LEAF-TABS table, #746 | H |
| `extension_extension_catalog`, `extension_custom_extensions_installer` | Go `extcatalog` (Python tab calls `bridge.extcatalog`); launcher Extensions page | wrapped | `extcatalog/INTEGRATION.md` | H |
| `extension_history_tab`, `extension_gallery_history` | `serverpy/history` (via bridge); Vite `/outputs/*` + `data-outputs` route | ported | `history/INTEGRATION.md`; `data_outputs.go` | H |
| `extension_log_viewer` | `serverpy/logview`; Vite `/tools/log_viewer` (`core-logs`) | ported | `logview/INTEGRATION.md`; LEAF-TABS | H |
| `extension_pip_list` | `serverpy/piplist`; Vite `/tools/pip_list` (`core-pip`) | ported | LEAF-TABS; `piplist/INTEGRATION.md` | H |
| `extension_model_location` | `serverpy/appenv` (`/api/env/model-locations`); Vite `/tools/model_location` | ported | LEAF-TABS decision 7 | H |
| `extension_gradio_settings` | `serverpy/appconfig`; Vite `/tools/server_settings` (7 options only) | ported | LEAF-TABS decision 4 | H |
| `extension_gpu_info` | `react_ui_backend/go_gpu_info.go` (`nvidia-smi`, NVIDIA only) | ported | LEAF-TABS decision 8 | H |
| `extension_ffmpeg_metadata` | `go_ffmpeg_metadata.go` (ffprobe) | ported | LEAF-TABS decision 9 | H |
| `extension_simple_remixer` | Vite `/tools/simple_remixer` (browser Web Audio, WAV out) | replaced | LEAF-TABS decision 11 | H |
| `extension_youtube_tutorials` | Vite `/tools/youtube_tutorials` (static) | ported | LEAF-TABS | H |
| `extension_minimax_cloud_tts` | none; Vite `/text-to-speech/minimax-tts` calls it through Gradio | not moved | LEAF-TABS decision 10 ("not part of the core") | H |
| `extension_conda_storage_optimizer` | none | not moved | LEAF-TABS "Left alone: undecided" | H |
| `extension_model_downloader`, `extension_huggingface_cache_manager` | none (P0 fixed the HF tab in place) | not moved | LEAF-TABS "class C"; `P0-BASELINE.md` fault 2 | H |
| `extension_whisper` | none | not moved | not mentioned in any rewrite doc; `#687` hardened it before the rewrite | M |
| `tts_webui_extension.openai_tts_api` (port 7778) | none; the `openai` platform can point at it as an endpoint | not moved | `openai/INTEGRATION.md` second demo; `all_tabs.py` still starts it | H |
| `tts_webui_extension.{f5_tts, rvc_training, ebook2audiobook, epub2tts, audiobook_generator, cuda_toolkit}` | none | not moved | no mention in any rewrite doc | M |
| `extension_decorator_save_ffmpeg` (decorator group) | none | not moved | `extensions.json` `decorators` | M |

### 3.5 react-ui: old Next.js pages vs Vite pages and Go routes (special case c)

| Old (Next, `2e57013`) | New | Relation | Evidence | Conf |
|---|---|---|---|---|
| `next.config.js`, `src/app/layout.tsx`, `pages/_app.tsx`, `_document.tsx`, `.eslintrc*`, jest and storybook files | `vite.config.ts`, `index.html`, `src/main.tsx`, `src/routes.tsx` (react-router) | replaced | #745, D21 | H |
| Next API routes `pages/api/gradio/[name].tsx` | `core/react_ui_backend/gradio.go` (`/api/gradio/{name}`) | ported | file header "Port of react-ui/.../[name].tsx" | H |
| `pages/api/gradio-file-proxy/[...path].tsx` | `gradio_file_proxy.go` | ported | CONTRACTS section 5 | H |
| `pages/api/webui-generations/[...name].ts` | `webui_generations.go` | ported | CONTRACTS section 5 | H |
| `app/api/upload/route.ts` | `upload.go` | ported | CONTRACTS section 5 | H |
| `getStaticProps` of `outputs/[name].tsx` | `data_outputs.go` (`/api/go-data/outputs/{name}`) + `useGoData` | replaced | CONTRACTS section 5 | H |
| `getStaticProps` of `bark/voices.tsx` | `data_bark_voices.go` | replaced | CONTRACTS section 5 | H |
| `pages/db.tsx`, `lib/databaseApi.ts`, `lib/sql.tsx` | none | dropped | `launcher/INTEGRATION.md` row "db" | H |
| `backend-utils/*` (defaultBackend, getFile, proxyGradioFile*) | logic in Go (`gradio.go`, `gradio_client.go`) | ported | deleted; `gradio.go` header | M |
| `next start` on port 3000 | Go listener serving `react-ui/out` (loopback, LAN opt-in) | replaced | D18, D20, D21 | H |
| Next dynamic/link/router/head APIs | `src/shims/*` | replaced | files added in #745 | H |
| `tools/gpu_info`, `outputs/ffmpeg-metadata-page` | same pages, now on `/api/go-data/gpu-info`, `/api/go-data/ffmpeg-metadata` | ported | LEAF-TABS | H |
| (new in Vite) `/tools/{pip_list,log_viewer,model_location,server_settings,extensions,simple_remixer,youtube_tutorials}` | Go routes `core-pip`, `core-logs`, `core-model-locations`, `core-settings`, `core-extensions`, none, none | new | LEAF-TABS table | H |
| (new) `/openai`, `/audiocpp`, `/onnx`, `/engines`, `/engine` | `core-openai`, `core-audiocpp`, `core-onnx`, `core-engines` | new | platform INTEGRATION files | H |
| Remaining generation pages (bark, chatterbox, kokoro, maha, mms, tortoise, vallex, minimax, musicgen, magnet, stable-audio, ace-step, demucs, rvc, vocos, pipeline, `outputs/*`, `extensions/*`, `index`) | same pages kept (50 files in `src/pages`), mostly unchanged, call `/api/gradio/<name>` | wrapped | `routes.tsx`; `grep` of `fetch("/api/gradio/` | H |

**Vite pages that still call Gradio api names through Go** (all via `POST /api/gradio/<name>`; Go calls the Gradio host
except the 7 rerouted names marked `engine`):

| Page / component | Api names | Where the tab lives now |
|---|---|---|
| `/text-to-speech/kokoro` | `kokoro_generate` | engine (own venv) |
| `/text-to-speech/mms` | `mms` | engine |
| `/text-to-speech/chatterbox`, `/tools/api_config` | `chatterbox_tts`, `get_chatterbox_voices` | engine |
| `/text-to-speech/vallex` | `vall_e_x_generate`, `..._split_text_into_sentences`, `..._tokenize` | engine |
| `/text-to-speech/bark*` | `bark`, `bark_favorite`, `bark_voice_generate`, `bark_voice_tokenizer_load`, `get_config_bark`, `save_config_bark`, `reload_old_generation_dropdown`, `encodec_decode`, `vocos_npz` | bark is own-venv (pre-P6), `vocos_npz` is P6 own-venv; **Go sends them to the host** |
| `/text-to-speech/maha-tts` | `maha`, `maha_tts_*`, `tortoise_open_voices` | maha own-venv (pre-P6) |
| `/text-to-speech/tortoise` | `tortoise`, `tortoise_*` | no `tortoise` entry in `extensions.json`: unknown |
| `/text-to-speech/minimax-tts` | `minimax_cloud_tts` | in-process host tab |
| `/audio-conversion/{demucs,rvc,vocos_wav}` | `demucs`, `rvc`, `rvc_*`, `vocos_wav` | P6 own-venv; **Go sends them to the host** |
| `/audio-music-generation/{ace-step,magnet,musicgen,stable-audio}` | `ace_step_infer`, `magnet`, `musicgen`, `stable_audio_generate` | ace_step is P6 own-venv; the others own-venv pre-P6; **Go sends them to the host** |
| `/outputs/*` cards | `delete_generation`, `open_folder`, `save_to_voices` | in-process history tab (Go history behind it) |
| `/extensions/huggingface_cache_manager` | `scan_huggingface_cache_api`, `delete_huggingface_cache_revisions` | in-process host tab |
| `/tools/api_config` | `open_ai_api_{load,save}_presets`, `open_ai_api_test_voice_preset` | in-process `openai_tts_api` |
| `/text-to-speech/*`, menu | hidden when `host-status` says not installed (P3) | `hostStatus.ts`, `Header.tsx` |

## 4. Platforms and new engines (new capability)

| New | Relation to old | Notes | Evidence |
|---|---|---|---|
| `platform/uv` (+ `tools`, `torchrows`) | replaces the micromamba env for engines | Linux git is the system's; mac has no pinned ffmpeg | `uv/INTEGRATION.md` |
| `platform/gradiohost` | replaces the shared env as host | optional entry | `gradiohost/INTEGRATION.md` |
| `platform/openai` | new | remote endpoints, key never returned, clips in History | `openai/INTEGRATION.md` |
| `platform/audiocpp` | new | audio.cpp v0.9.6 pinned, Windows only | `audiocpp/INTEGRATION.md` |
| `platform/onnx` | new | sherpa-onnx v1.13.8, Windows x64 CPU, Kitten nano | `onnx/INTEGRATION.md` |

## 5. Lists

### 5.1 Old files and modules with no new counterpart (P9 removal candidates)

`legacy-env` is kept "until the maintainers say it can go (PLAN.md, P9)" (`MIGRATION.md`). Nothing below was tested for
removal. "Still depends" is what a grep or a doc shows.

| Old item | Still depended on by | Notes |
|---|---|---|
| `installer_scripts/init_mamba.sh/.bat`, `init_app.js`, `js/*` (except what Go reads), `root.*`, `tools/*` | `start_tts_webui.*`; `TTS_GO_STEPS=none`; legacy-env installs; `go` ports still read `versions.json` and `js/initializeApp.js` | Go has ports of all; the scripts are the escape hatch (D19) |
| `installer_scripts/js/server.js`, `webConsole.js`, `installerState.js`, `menu.js`, `openBrowser.js`, `index.html` | only commented-out lines in `start_app.*` | Looks unreferenced; port 7771 listed in `CALL_TREE.md` |
| `installer_files/env` creation (micromamba) | `legacy-env`, MIGRATION.md | Step set `mamba, gpu, install` |
| `tts_webui/database/*` | no importer found in repo | Check the `openai_tts_api` extension (not in this repo) before removing |
| `tts_webui/cli.py` and the `tts-webui` entry in `pyproject.toml` | `pip install` users; `tests/test_cli_integration.py` | Name clashes with the Go exe |
| `tts_webui/gradio/*`, `decorators/*`, `utils/*`, `config/*`, `core_bridge/*`, `extensions_loader/*` | The Gradio host and every in-process tab | Not removable while the host exists |
| `extensions/builtin/*` tabs with Go counterparts (pip_list, log_viewer, model_location, gradio_settings, gpu_info, ffmpeg_metadata, remixer, youtube_tutorials, lists, history) | Gradio host users; `gradio-host` base requirements | LEAF-TABS: "Removing the old tabs is a later step" |
| `requirements.txt` lines for demucs, mms, seamless_m4t, vocos, vall_e_x, log_viewer, openai_tts_api | `gradio-host` install and legacy installs | 5 of these 7 are now own-venv entries (see section 7) |
| `tests/` (Python) | `tests.yaml` CI | cover Python code that still exists |
| Next-era sections of `CONTRACTS.md` (sections 4-8, 10) and `core/README.md` ("exe only launches", micromamba setup) | readers | Docs, not code; stale |

### 5.2 New files and modules with no old counterpart (new capability)

| New | What it adds |
|---|---|
| `core/internal/platform/{requires,detect}.go` and the `Platform` seam | `requires` graph, install plans, dependants check, per-install platform choice |
| `platform/uv`: pinned uv, tools provider (git, ffmpeg), same-volume hardlink cache, `entry.json` state, pre-flight (path length, free disk) | Reproducible engine venvs sharing torch files |
| `platform/openai`, `platform/audiocpp`, `platform/onnx` and Vite pages `/openai`, `/audiocpp`, `/onnx` | Remote, native and ONNX engines with no Python |
| `launcher/archive.go`, `selfswap.go` | Archive update and self-replacing exe |
| `launcher/engine_limits.go`, `extprocs/limits.go` | Idle stop and resident cap |
| `launcher/nohost.go`, `hoststatus.go`, `lan_status.go`, `tlslisten.go`, `orphans_*`, `procscan_*` | No-host page, host status, LAN mode, TLS on the router, orphan sweep |
| `react_ui_backend/proxy*.go`, `static_export.go`, `core_client.go` | Go front door on 3000: Host/Origin policy, LAN opt-in, static export build checks |
| `core/scripts/smoke/*`, `engine-check.ps1`, `ci/install-host.sh` | Tiered engine checks, key-leak smoke, install-host CI |
| `core/ui/src/core-app/*` | Launcher UI: Setup, Extensions (GPU panel, plans, cancel), notices |
| `react-ui` `/engines`, `/engine` pages and `serverpy/engines` | Open an own-venv engine's Gradio page from Vite |

## 6. Not verified or open (carried from the INTEGRATION files)

- P5: no real remote OpenAI endpoint or key was used (Rob accepted this on 2026-11-25).
- P6: only 3 of 20 engines have `validation: full`; the other 17 passed install and start only.
- P7, P8: Windows only; Linux and macOS pins refused. P7 stop-process cancel fallback only against a fake server.
- P2/P3: macOS never run; Linux install steps through the API, not clicks; uv platform uninstall and cache clearing not built.
- P1: `--system-python` only in unit tests; the Docker image was never built (P4 notes: no daemon).
- CI workflows from P4 written but not run on GitHub.

## 7. Unknown or contradictory

1. **Brief says 46 in-process tabs and 20+ own-venv entries; HEAD has 26 and 40.** The 46/20 is the P1-P5 state.
   `MIGRATION.md` still says "46 in-process tabs", which is now out of date.
2. **`requirements.txt` still installs demucs, mms, seamless_m4t, vocos, vall_e_x into the host,** and `hostBaseTabs`
   (`extcatalog/host.go:132`) reads that file to decide what counts as a base tab. Those five are now `native` own-venv entries.
   What happens when both exist: unknown (not run).
3. **Seven api names are rerouted; 13+ other pages for own-venv engines still call the host.** For bark, maha, musicgen,
   magnet, stable_audio, ace_step, rvc, demucs, vocos, `encodec_decode`: whether `/api/gradio/<name>` still works when
   the engine's tab no longer lives in the host: unknown. No test or doc covers it; `engine-check.ps1` has calls for 4.
4. **`tortoise_*` api names:** `extensions.json` has no tortoise entry. Where the tab comes from: unknown.
5. **`PLAN.md` in the repo is the ring plan (rings 1-7, D1-D21).** The P0-P9 items and decisions D23-D32 that
   PLATFORM-DESIGN and the INTEGRATION files cite are not in the repo (`DECISIONS.md` stops at D21). P9 is only named in
   `MIGRATION.md`. What P9 contains: unknown.
6. **`CONTRACTS.md` (480 lines) describes Next-era behaviour** (TTS_GO_PROXY=0, `next start`, side-by-side
   harness) that D21 removed; the file's banner says so, but the sections are not trimmed.
7. **`core/README.md`** says the exe only launches and the setup page runs micromamba; still partly true only for legacy installs.
8. **`start_tts_webui.*` runs `init_mamba`** (micromamba) although P4 made a micromamba-free default install. Whether the
   release zip ships these scripts as the main entry: unknown.
9. **`extension_whisper`, `decorator_save_ffmpeg`, `f5_tts`, `rvc_training`, `ebook2audiobook`, `epub2tts`,
   `audiobook_generator`, `cuda_toolkit`:** no rewrite doc mentions them; relation recorded as "not moved" from absence only.
10. **`installation.html`, `outputs-rvc/`, `collections/`, Colab notebook, `tts-webui-ignition` (Neutralino):** what uses
    them now: unknown.
11. **`tts_webui/database` / `TTS_WEBUI_DB_PATH`:** Go never starts the DB API (`core_server.py` says so), but the old installer
    step still configures it, and the `openai_tts_api` extension is outside this repo. Whether anything consumes it: unknown.
12. **P0 and P1-P3 tables report "112 tabs" in a real Gradio install** while the catalog has 66-68 entries. How the 112 is
    counted (sub-tabs, groups, decorators): unknown.

## 8. What was not checked

- No code ran: no `go test`, no app start, no installs, no browser. All "works" statements are quoted from the INTEGRATION
  and baseline docs, not reproduced.
- Not read file by file: `tts_webui/utils/*`, `tts_webui/decorators/*`, `core/react_ui_backend` handlers beyond headers and
  patterns, `react-ui/src/components`, `core/ui` sources, the Go tests, `.github/workflows` content.
- `git log` dating covers the PR merges listed in section 1; per-file dates are given only where a deletion commit was read.
- Branches other than `main` (`core/rewrite`, `core/go-port`, `platform/*`) and `.claude/reports/*` (LEAF-TABS analysis) are
  not in this checkout and were not read.
- PRs #759 and later were not examined.
