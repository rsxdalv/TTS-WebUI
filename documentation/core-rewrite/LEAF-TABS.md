# Leaf tabs: Go routes and Vite pages

Branch `claude/leaf-tabs-convert`. Analysis: `.claude/reports/leaf-tabs.md` (LAB-36). Goal (Rob): the Go core boots and
runs by itself; nothing needed to boot may need Python. The Python extensions and Gradio tabs are unchanged; the new
pages sit beside them. Removing the old tabs is a later step.

## Baseline: what the core does today with no Python env and no Gradio child

Read from the code on origin/main b476228; the first point was run for real.

1. **Verified (real run, empty root with only `react-ui/out`, `tts-webui launch --start --no-browser`).** The launcher
   page on 7700 comes up. The app does **not** start: `determining GPU: GPU label not set: complete the GPU step first`
   (`installer_scripts/.gpu` is missing). `--system-python` skips that gate. Nothing on :3000 was started by this run
   (a different app was already listening on 3000 and 7770 on the test machine, so I did not go further with a real
   start there).
2. **Unverified (from `supervise/supervisor.go` and `launcher/serverpy.go`).** Once past the gate:
   - The Gradio child is the first process of `Start` (`defs`), and a failure to start it fails the whole `Start`:
     the React front is closed again and :3000 is never served. A micromamba or env that is missing makes that start
     fail.
   - The Go side (router, control API on a random loopback port, config/env/logs/pip/extcatalog services) is started
     by `startServerPy` and does not itself need Python.
   - The control API is closed together with the Gradio child (`closingRunner`). If Gradio starts and dies (no
     Python in the env), :3000 stays up but the control API is gone, so every route that needs it answers 503.
   - The React front serves the static export and is already independent of Python.
3. **So the blocker for "boots with no Python" is the supervisor, not these routes:** the Gradio child is mandatory,
   and the control API lives and dies with it. That is the launcher's part (`supervise`, `launcher/serverpy.go`); I did
   not change it. These routes are written so that they work once it is fixed (see "Without Python", below).

## What exists now

| Tab | Page (react-ui) | Route (core/react_ui_backend) | Backend | Works without Python? |
|---|---|---|---|---|
| pip_list | `/tools/pip_list` | `GET /api/go-data/pip-list` (`core-pip`) | piplist (runs pip) | **No, by nature.** Answers 409 `no Python environment`; the page says so |
| log_viewer | `/tools/log_viewer` | `GET/POST /api/go-data/logs/{op}` (`core-logs`) | logview | Yes (empty list when there are no logs) |
| model_location | `/tools/model_location` | `GET/PUT /api/go-data/model-locations` (`core-model-locations`) | appenv (new `GET /api/env/model-locations`) | Yes |
| gradio_settings | `/tools/server_settings` | `GET/PUT /api/go-data/server-settings` (`core-settings`) | appconfig `replace-section` | Yes |
| extension lists (read only) | `/tools/extensions` | `GET /api/go-data/extensions/{kind}` (`core-extensions`) | extcatalog `list` | Yes (tested with a missing env) |
| youtube_tutorials | `/tools/youtube_tutorials` | none (static) | none | Yes (needs the internet for the videos) |
| gpu_info | existing `/tools/gpu_info` and the Bark settings widget, now on Go | `GET /api/go-data/gpu-info` (`go-gpu-info`) | nvidia-smi | Yes; `[]` and a "No NVIDIA GPU found" message without nvidia-smi |
| ffmpeg_metadata | existing `/outputs/ffmpeg-metadata-page`, now on Go | `POST /api/go-data/ffmpeg-metadata` (`go-ffmpeg-metadata`) | ffprobe | Yes if ffprobe exists, else 503 `ffprobe was not found ...` |
| minimax_cloud_tts | existing `/text-to-speech/minimax-tts`, now on Go | `POST /api/go-data/minimax-tts` (`go-minimax-tts`) | MiniMax API | Yes (400 without a key) |
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
9. **ffmpeg_metadata.** `{name}` of a file in `react-ui/public/file-input-cache` (what `/api/upload` fills), never a
   path. Same tag rules as the extension; tag names matched without case; a file with no usable comment tag returns
   the whole ffprobe document instead of an error.
10. **minimax.** Same API request and files as the extension (`outputs/<timestamp>_<text>/<text>.mp3` and `.json`).
    Key: request, else `MINIMAX_API_KEY`; never logged, written or returned, and masked in error texts. A name clash
    gets a `_2` suffix instead of overwriting.
11. **simple_remixer.** Same rule as the extension (mix each column, join the columns), mono; the rate is the browser
    AudioContext's (the extension used the highest rate of its slots). Output is 16-bit WAV.
12. **youtube_tutorials** uses `youtube-nocookie.com`. No CSP is set by the Go server (nothing in front of it checked).
13. **gosrv** (dev server) got `-core-url` / `-core-token` to try the routes against a `pipd` or `logd`.

## Left alone

- `conda_storage_optimizer`: undecided (likely does nothing on a micromamba install). Not converted.
- `model_downloader`, `huggingface_cache_manager`, `demucs`, `vocos`: class C, native extensions stay.
- The launcher/supervisor: see the baseline. Not changed.

## Checked and not checked

Checked: `go vet ./...` and `go test ./...` in `core`; `node build-static.js`; `tsc` shows no errors in the new files;
real pip list through `gosrv` and a real `pipd` in the browser pane; `/api/go-data/gpu-info` against the real
`nvidia-smi` of this machine (GTX 1070); the real services behind the routes in `no_python_test.go`; the remixer
mixing code with node; the baseline GPU-gate failure with a real launcher run.
Not checked: a launcher start past the GPU gate (a different app holds ports 3000 and 7770 here), so the supervisor
behaviour in point 2 of the baseline is from code only; a real MiniMax call; a real ffprobe run through the route
(only a stub); the remixer with real audio in a browser; the GPU page rendered in a browser; the Gradio tabs next to
the new pages.
