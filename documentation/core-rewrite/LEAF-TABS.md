# Leaf tabs: Go routes and Vite pages

Branch `claude/leaf-tabs-convert`. Analysis: `.claude/reports/leaf-tabs.md` (LAB-36). The Python extensions and
Gradio tabs are unchanged; the new pages sit beside them. Removing the old tabs is a later step.

## What exists now

| Tab | Page (react-ui) | Route (core/react_ui_backend) | Backend |
|---|---|---|---|
| pip_list | `/tools/pip_list` | `GET /api/go-data/pip-list` (`core-pip`) | piplist |
| log_viewer | `/tools/log_viewer` | `GET/POST /api/go-data/logs/{op}` (`core-logs`) | logview |
| model_location | `/tools/model_location` | `GET/PUT /api/go-data/model-locations` (`core-model-locations`) | appenv (new `GET /api/env/model-locations`) |
| gradio_settings | `/tools/server_settings` | `GET/PUT /api/go-data/server-settings` (`core-settings`) | appconfig `replace-section` |
| extension lists (read only) | `/tools/extensions` | `GET /api/go-data/extensions/{kind}` (`core-extensions`) | extcatalog `list` |
| youtube_tutorials | `/tools/youtube_tutorials` | none (static) | none |
| ffmpeg_metadata | existing `/outputs/ffmpeg-metadata-page`, now on Go | `POST /api/go-data/ffmpeg-metadata` (`go-ffmpeg-metadata`) | ffprobe |
| minimax_cloud_tts | existing `/text-to-speech/minimax-tts`, now on Go | `POST /api/go-data/minimax-tts` (`go-minimax-tts`) | MiniMax API |
| simple_remixer | `/tools/simple_remixer` | none | Web Audio in the browser |

All pages are in the Tools menu (Header.tsx).

## Decisions

1. **Token.** The launcher control API (random loopback port, `X-TTS-Core-Token`) is reached through
   `react_ui_backend.Core`. The supervisor creates it before the React front starts and fills it in once the Go side
   of Gradio is up (`GradioSide.CoreURL/CoreToken`). Until then the routes answer 503. The browser never sees the
   token; a token header sent by the browser is not forwarded.
2. **CSRF / same origin (decided once, `core_client.go`).** Port 3000 has no token (CONTRACTS section 6). The front
   door already checks Host on every request and Origin on every `/api` request and every non-GET. Every route that
   changes something also requires `Content-Type: application/json`. Tests: `TestMutationsRefuseOtherOrigins`,
   `TestLogsOperations`.
3. **Operation allow-lists.** `logs/{op}` forwards only the seven read and two delete operations, so no other
   `/api/logs` path is reachable. The upstream still validates file names and parameters.
4. **gradio_settings.** Only the options the Go core reads are offered: `server_name`, `server_port`, `auth`,
   `ssl_keyfile`, `ssl_certfile`, `ssl_keyfile_password`, `ssl_verify`. The Gradio-only options (inline, share,
   show_api, max_threads, ...) are not offered and stay in the file untouched: a save reads the section, changes
   the given keys, and writes it back whole with `replace-section`. Secrets never leave Go: GET returns the user
   names and whether a key password is set; PUT keeps what it is not given; `null` removes. `auth` is stored as
   `[user, password]`, so a password with `:` works. Known gap: the read and the write are two calls, so a change
   between them by another writer of that section is lost.
5. **Restart button.** Left out. The launcher's restart is on port 7700 with its token, which CONTRACTS section 6
   keeps off port 3000 ("a claim must not start processes or change the install"). The page says to restart from
   the launcher.
6. **Extensions.** Only the lists are served. Install, uninstall, dependency checks, latest-version and "add
   entries" start pip/uv or change the install, and `install-entry` takes requirements from pasted text; they stay
   in the Gradio tabs and the launcher's Extensions page. The four management tabs are therefore only partly
   covered: the NDJSON stream reader is not built.
7. **model_location** writes `.env` through the existing appenv call (the same one the Python tab used). The page
   shows what `.env` says, not the process environment.
8. **ffmpeg_metadata.** The route takes `{name}` of a file in `react-ui/public/file-input-cache` (what `/api/upload`
   fills), never a path. Same tag rules as the extension; tag names are matched without case; a file without a
   usable comment tag returns the whole ffprobe document instead of an error. ffprobe is looked up in
   `installer_files/env` and on PATH; 503 if absent.
9. **minimax.** Same API request and same files as the extension (`outputs/<timestamp>_<text>/<text>.mp3` and
   `.json`, same keys). Key: request, else `MINIMAX_API_KEY`; it is never logged, written or returned, and is
   masked in error texts. A name clash gets a `_2` suffix instead of overwriting. The React page now calls the Go
   route; it throws the route's error text.
10. **simple_remixer.** Same rule as the extension (mix each column, join the columns), mono. Rate is the browser
    AudioContext's rate (the extension used the highest rate of its slots). Output is 16-bit WAV.
11. **youtube_tutorials** uses `youtube-nocookie.com`. The Go server sets no CSP of its own (not checked for
    anything in front of it).
12. **gosrv** (dev server) got `-core-url` / `-core-token` to try the routes against a `pipd` or `logd`.

## Left alone

- `conda_storage_optimizer`: undecided (it likely does nothing on a micromamba install). Not converted.
- `model_downloader`, `huggingface_cache_manager`, `demucs`, `vocos`: class C, skipped.
- `gpu_info`: the page already exists and works through the Gradio passthrough; no Go replacement yet.

## Checked and not checked

Checked: `go vet ./...` and `go test ./...` in `core`; `node build-static.js` in `react-ui`; `tsc` shows no errors in
the new files; the pip list page against a real `pipd` (Python 3.12) through `gosrv`; the mixing code of the remixer
with node. Not checked: a full launcher start with Gradio (no install here), the log, model-location, settings and
extension routes against the real services (only fake-core tests; the services have their own tests), a real
MiniMax call, a real ffprobe run, the remixer with real audio in a browser, and the Gradio tabs next to the new pages.
