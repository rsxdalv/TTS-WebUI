# Leaf tabs: Go routes and Vite pages

Branch `claude/leaf-tabs-convert`. Analysis: `.claude/reports/leaf-tabs.md` (LAB-36). Goal (Rob): the Go core boots and
runs by itself; nothing needed to boot may need Python. The Python extensions and Gradio tabs are unchanged; the new
pages sit beside them. Removing the old tabs is a later step.

## Baseline (before) and what changed: the core with no Python env and no Gradio child

**Before (origin/main b476228).**

1. Verified by a real run (empty root with only `frontend/out`, `tts-webui launch --start --no-browser`): the launcher
   page comes up, the app does not start: `determining GPU: GPU label not set: complete the GPU step first`
   (`installer_scripts/.gpu` missing). `--system-python` skipped that gate.
2. Verified by the tests that were on origin/main (`TestStartErrorMarksFailed`, `TestStartFailureClosesTheReactFront`):
   a Gradio child that could not start made `Start` fail with `starting gradio: ...` and closed the React front, so
   :3000 was never served.
3. From the code only (not run before the change): the control API and services were closed together with the Gradio
   child (`closingRunner` closers), so a Gradio that died left :3000 up with every core route at 503.

**After (this branch, commit "Go core boots without Gradio").** All of this is verified by tests that run the
launcher's real supervisor wiring (`launcher/gocore_test.go`) or the supervisor with a fake side
(`supervise/gocore_test.go`); see "Checked and not checked" for what was not run.

- The Go side (router, control API, config/env/logs/pip/extcatalog services) and the React front start first. The Go
  side is held by its own runner (`NameCore`, not in the status) until `Stop` or the next `Start`; it is not closed when
  Gradio exits or fails to start. A `Start` after Gradio died with nothing else alive closes the old Go side first
  (`releaseCore`), so the ports can be bound again.
- Gradio is optional. `pythonAvailable` checks the command prefix: the program (`micromamba`) must exist and the
  `-p <env>` directory must exist (system Python: `python` on PATH). If not, Gradio is not started and its status is
  `failed` with `no Python environment: ... (the install has not been done)`. If it cannot start for another reason, or
  exits later, it is `failed` / `exited` with the exit code and log tail. None of these fails `Start`. A failure of the
  Go side itself still does.
- The launcher page now shows each process's state and error, not only running/stopped (`LaunchStep.tsx`).
- The browser: Gradio's own tab opens only when Gradio becomes ready; the React tab opens either way.
- **GPU label decision (Rob): not required for now.** A missing or invalid `installer_scripts/.gpu` never stops `Start`.
  It is a warning: a log line and `warning` on the gradio status, which the launcher page shows, for example
  `GPU label not set: ... Carrying on without a GPU label: Gradio and the Go core do not use it; installing a native
  extension's torch needs it and will say so`. The check lives in one function, `Supervisor.checkGPULabel`; it is skipped
  when `SkipGPUCheck` is set (system Python) or Gradio cannot start. What nothing started by `Start` reads the label:
  Gradio, the Go side and the native extension processes do not. The only consumer is the install of a native
  extension's torch (`extcatalog/torch.go`), which keeps failing with its own clear message ("GPU choice file not
  found ... run the installer") instead of silently installing CPU torch; that is the label-dependent part that is
  skipped. Tests: `TestGPULabelIsAWarning`, `launcher/TestMissingGPULabelIsOnlyAWarning`.
  **Revisit:** set `Config.StrictGPULabel` (not wired to any flag yet) to make it a hard stop again
  (`TestStrictGPULabelStopsStart`); decide then whether native installs should fall back to CPU torch.
- Still needed to boot: the static export in `frontend/out` (or npm to build it, which today comes from the env), and
  `config.json` (a missing one means defaults). Not changed.
- On the public port (default 7770) the Go router answers 502 for `/` while Gradio is down; the React UI on :3000 is
  the app in that case.

## What exists now

| Tab | Page (react-ui) | Route (core/react_ui_backend) | Backend | Works without Python? |
|---|---|---|---|---|
| pip_list | `/tools/pip_list` | `GET /api/go-data/pip-list` (`core-pip`) | piplist (runs pip) | **No, by nature.** Answers 409 `no Python environment`; the page says so |
| log_viewer | `/tools/log_viewer` | `GET/POST /api/go-data/logs/{op}` (`core-logs`) | logview | Yes (empty list when there are no logs) |
| model_location | `/tools/model_location` | `GET/PUT /api/go-data/model-locations` (`core-model-locations`) | appenv (new `GET /api/env/model-locations`) | Yes |
| gradio_settings | `/tools/server_settings` | `GET/PUT /api/go-data/server-settings` (`core-settings`) | appconfig `replace-section` | Yes |
| extension lists, with install and uninstall (the Gradio list tabs are retired) | `/tools/extensions` | `GET /api/go-data/extensions/{kind}`, `.../{kind}/state` (`core-extensions`), `POST .../install` and `.../uninstall` (`core-extensions-install`) | extcatalog `list`, `resolve`, `install`, `uninstall` | Yes (tested with a missing env) |
| youtube_tutorials | `/tools/youtube_tutorials` | none (static) | none | Yes (needs the internet for the videos) |
| gpu_info | existing `/tools/gpu_info` and the Bark settings widget, now on Go | `GET /api/go-data/gpu-info` (`go-gpu-info`) | nvidia-smi | Yes; `[]` and a "No NVIDIA GPU found" message without nvidia-smi |
| ffmpeg_metadata | existing `/outputs/ffmpeg-metadata-page`, now on Go | `POST /api/go-data/ffmpeg-metadata` (`go-ffmpeg-metadata`) | ffprobe | Yes if ffprobe exists, else 503 `ffprobe was not found ...` |
| minimax_cloud_tts | existing `/text-to-speech/minimax-tts`, unchanged (Gradio) | none | Python extension | No (not core; see decisions) |
| simple_remixer | `/tools/simple_remixer` | none | Web Audio in the browser | Yes |

All pages are in the Tools menu (Header.tsx).

## Without Python

- Every new route answers a plain state with JSON `{"error": "..."}` and never waits on Python or Gradio:
  503 `the core API is not available` when the control API is not up (its token is not set yet, or it was closed with
  the Gradio child), 409 `no Python environment` for pip list, 503 for a missing ffprobe, `[]` for no GPU.
- `no_python_test.go` starts the React front with no Gradio backend and the real Go services (config, env, logs, pip,
  extcatalog) over an environment that does not exist, with an empty PATH, and calls every new route (15 s client
  timeout, so a hang fails). A second test does the same with no control API at all. Both pass.
- `piplist` got `ErrNoPython`: no `Python` option on PATH, no Run program, or a `-p <env>` directory that does not
  exist. This is a state, not a failure.
- **Installer note, ffprobe.** The micromamba env installs `ffmpeg=4.4.2` (`installer_scripts/init_mamba.go`), which
  brings `ffprobe` (seen at `installer_files/env/Library/bin/ffprobe.exe`); Go looks there (`Library/bin`, `bin`,
  `Scripts`), then on PATH. That install needs micromamba, not Python. A core that runs with no env at all needs
  the installer to either create an env with only `conda-forge::ffmpeg` (no python), or put a static `ffprobe` into
  `installer_files/env/bin` (or `Library/bin` on Windows). Until then the route says `ffprobe was not found (install
  ffmpeg or add ffprobe to PATH)` and the tab is not dropped.

## Decisions

1. **Token.** The launcher control API (random loopback port, `X-TTS-Core-Token`) is reached through
   `react_ui_backend.Core`. The supervisor creates it before the React front starts and fills it in once the Go side
   of Gradio is up (`GradioSide.CoreURL/CoreToken`). The browser never sees the token; a token header sent by the
   browser is not forwarded.
2. **CSRF / same origin (decided once, `core_client.go`).** Port 3000 has no token (CONTRACTS section 6). The front
   door already checks Host on every request and Origin on every `/api` request and every non-GET. Every route that
   changes something also requires `Content-Type: application/json`. Tests: `TestMutationsRefuseOtherOrigins`,
   `TestLogsOperations`.
3. **Operation allow-lists.** `logs/{op}` forwards only the seven read and two delete operations. The upstream still
   validates file names and parameters. All its answers are JSON (text ones are JSON strings); the page parses them.
4. **gradio_settings.** Only the options the Go core reads are offered: `server_name`, `server_port`, `auth`,
   `ssl_keyfile`, `ssl_certfile`, `ssl_keyfile_password`, `ssl_verify`. The Gradio-only options (inline, share,
   show_api, max_threads, ...) are dropped from the page and stay in the file untouched (a save reads the section,
   changes the given keys, writes it back whole). Secrets never leave Go: GET returns user names and whether a key
   password is set; PUT keeps what it is not given; `null` removes. `auth` is stored as `[user, password]`. Known gap:
   read and write are two calls, so a change in between by another writer of that section is lost.
5. **Restart button.** Left out. The launcher's restart is on port 7700 with its token, which CONTRACTS section 6
   keeps off port 3000. The page says to restart from the launcher.
6. **Extensions.** Only the lists are served. Install, uninstall, dependency checks, latest-version and "add
   entries" start pip/uv or change the install, and `install-entry` takes requirements from pasted text; they stay
   in the Gradio tabs and the launcher's Extensions page. The four management tabs are only partly covered; the
   NDJSON stream reader is not built.
7. **model_location** writes `.env` through the existing appenv call. The page shows what `.env` says, not the process
   environment.
8. **gpu_info** from `nvidia-smi --query-gpu=...` (found on PATH, then the usual install paths), cached 1 s because the
   widget polls every 2 s. Same keys as `get_gpu_info`. Neutral values for what only torch knew: `torch_version` "",
   `used_vram` and `cached_vram` 0, `multi_processor_count` 0; `used_vram_total` is the device memory in use;
   `cuda_version` is the driver's CUDA version from the nvidia-smi header. NVIDIA only; AMD, Intel and Apple get `[]`.
9. **ffmpeg_metadata.** `{name}` of a file in `frontend/public/file-input-cache` (what `/api/upload` fills), never a
   path. Same tag rules as the extension; tag names matched without case; a file with no usable comment tag returns
   the whole ffprobe document instead of an error.
10. **minimax: not converted.** A Go handler was written and then dropped (Rob: MiniMax is not part of the core). The
    Python extension and the React page that calls it through Gradio are as they were; it works only while Gradio runs.
    Later: a Go backend for the page was written on its own branch, outside the platform packages; see `EXT-MINIMAX.md`
    (draft; the Python extension and the Gradio tab are unchanged there too).
11. **simple_remixer.** Same rule as the extension (mix each column, join the columns), mono; the rate is the browser
    AudioContext's (the extension used the highest rate of its slots). Output is 16-bit WAV.
12. **youtube_tutorials** uses `youtube-nocookie.com`. No CSP is set by the Go server (nothing in front of it checked).
13. **gosrv** (dev server) got `-core-url` / `-core-token` to try the routes against a `pipd` or `logd`.

## Left alone

- `conda_storage_optimizer`: removed (see [EXT-D.md](EXT-D.md)); it had no replacement because `conda clean --all` means nothing on the uv platform.
- `model_downloader`, `huggingface_cache_manager`, `demucs`, `vocos`: class C, native extensions stay.
- The launcher and supervisor were changed for the Go core to boot without Gradio (see the baseline section).

## Checked and not checked

Checked: `go vet ./...` and `go test ./...` in `core`; `node build-static.js`; `tsc` shows no errors in the new files;
real pip list through `gosrv` and a real `pipd` in the browser pane; `/api/go-data/gpu-info` against the real
`nvidia-smi` of this machine (GTX 1070); the real services behind the routes in `no_python_test.go`; the remixer
mixing code with node; the baseline GPU-gate failure with a real launcher run.
Also checked: the supervisor with a Gradio that cannot start, one that dies later, no Python env with no GPU label, and
a second `Start` after Gradio died (`supervise/gocore_test.go`); the launcher's real wiring on a temp install with no
env (React front, real control API and services answering, Gradio reported as failed) and with a Gradio that exits with
code 3 (control API still answering) (`launcher/gocore_test.go`); `tsc` on the launcher UI change.
Real run (sub session, built binary, empty scratch root with only `frontend/out` and a `config.json`; no env, no
`.gpu`): `tts-webui launch --start --no-browser` started. The launcher status showed gradio `failed` with `no Python
environment: ...micromamba.exe was not found (the install has not been done)` and react-ui `ready`. :3000 served the
UI (200); `pip-list` 409 `no_python`; `logs/files` 200 `[]`; `model-locations`, `server-settings` (no secrets) and
`gpu-info` (the real GTX 1070) 200; `extensions/interface` 200 `[]` (no catalog file in the scratch root); the router on
the public port answered 502 `upstream unavailable`. No warning shows without an env, because the GPU label is only
checked when Gradio could start. The launcher wrote a default `.env` into the root.
Not checked: a start with a real Python env after the change (the opt-in `LAUNCHER_REAL_ROOT` tests were not run); the
launcher page and the new pages (other than pip list) rendered in a browser; a real ffprobe run
through the route (only a stub); the remixer with real audio; the Gradio tabs next to the new pages; a no-env start when
`frontend/out` is missing (it needs npm to build it).
