# extprocs: wiring (stage 4)

Go runs the proxy-extension processes; `tts_webui/core_ext_bridge` makes
`setup_proxy_extension.py` ask Go instead of spawning them itself. No
existing file was edited.

## Changes Rob must make in existing files

1. **`core/internal/launcher/supervisor.go`, `Supervisor.Start`**
   - Pick a control address (the launcher's own HTTP server) and a random token.
     Pass to the gradio child env: `TTS_CORE_URL=http://127.0.0.1:<port>`,
     `TTS_CORE_TOKEN=<token>` (plus stage 1/2/3 vars).
   - Build the manager once per app run:
     ```go
     mgr := extprocs.New(extprocs.Options{
         Root:       s.root,
         MainPython: []string{mamba, "-p", envPath, "run", "python"}, // same activation as server.py
         BaseEnv:    baseEnv,   // may contain TTS_CORE_*; they are stripped for extensions
         Routes:     rtr,       // stage 3's *router.Router (satisfies extprocs.RouteRegistrar)
         Token:      token,
     })
     ```
   - `Supervisor.Stop`: call `mgr.StopAll()` (blocks until trees are killed).
2. **Launcher HTTP mux** (same listener `TTS_CORE_URL` points at; must be loopback):
   ```go
   mux.Handle("/api/extensions", mgr.Handler())
   mux.Handle("/api/extensions/", mgr.Handler())
   ```
3. **Entry script**: run `server_core.py` (stage 3) instead of `server.py`; it calls
   `tts_webui.core_ext_bridge.install()` (no-op without `TTS_CORE_URL`).

## Behaviour notes

- API (loopback + `X-TTS-Core-Token`, POST needs `application/json`; else 403/415):
  `POST /api/extensions/{pkg}/start` `{"port","openai_proxy_host","env"}` -> `{"port","pid"[,"already_running"]}`;
  `POST .../stop` (404 if never started); `GET /api/extensions`;
  extra: `GET /api/extensions/{pkg}/logs?tail=N` -> `{"lines":[...]}`.
- Contract extension: optional `env` in the start body. Python sends only the
  variables whose value changed since `install()` (e.g. `.env`/`.env.user`
  values loaded by `dotenv_init.init()`, `GRADIO_TREE_*`), because extensions
  inherited these from server.py's `os.environ` before. `TTS_CORE_TOKEN` and
  `TTS_CORE_URL` are never passed to extension processes.
- `Options.ExtraEnv` (optional) returns `KEY=VALUE` variables Go computes for
  each extension at start; they win over `BaseEnv` and the request's `env`. The
  launcher uses it for `TTS_WEBUI_DECORATORS` (the resolved decorators; see
  extcatalog), because extension processes have no control token. If it fails,
  the extension is not started.
- Go registers `/<pkg>` itself once the port accepts TCP (so a restart after
  Stop gets its route back; Python's `add_route_once` is cached and only runs
  once). Python's own `add_extension_route` call (stage 3) is then a harmless
  idempotent re-add.
- If Go has to use another port than Python asked for (requested port busy,
  or package already running from a previous server.py run), the shim
  translates `wait_for_server` and `add_extension_route` to Go's port; this
  is the only other thing patched in that module.
- ReadyTimeout (default 120s): state becomes `failed` but the process keeps
  running and probing; if it comes up later it turns `ready` and is routed.
- The bridge patches `setup_proxy_extension` through an import hook when it is
  first imported, not by importing it in `install()`: importing it loads gradio
  before `dotenv_init.init()`, which changes when gradio/huggingface_hub read `.env`.
- Errors from Go surface as `gr.Error` with the reason (also on stderr as
  `[tts-core] ...`); stop treats 404 as already stopped.

## Verified for real vs only in tests

Real:
- `go test ./internal/serverpy/extprocs/...` and `go vet` pass on Windows
  (go1.27.4): fake python built from `testdata/fakepy` with real processes, ports, job-object kill.
- `TestRealExtension` (opt-in: `EXTPROCS_REAL_PYTHON`, `EXTPROCS_REAL_ROOT`) started the
  installed `tts_webui_extension.log_viewer` through the Manager using the main env's
  `python.exe`: reached `ready`, route registered, `GET /<pkg>/` returned 200, `Stop`
  removed the route. No `.venvs/<pkg>` extension is installed, so the venv-python path was
  only tested with the fake.
- Python: `python -m unittest tts_webui.core_ext_bridge.test_core_ext_bridge` (18 tests)
  incl. importing the real `setup_proxy_extension` in a child interpreter and
  confirming `install()` patched it and Popen/terminate hit the stub API.

Real, through the launcher (step 27, `launcher/real_serverpy_test.go`):
- `TestRealExtensionProcessAppliesDecorators`: `startServerPy` with the real
  `micromamba -p <env> run` prefix as `MainPython`, the real control API, and
  `tts_webui_extension.mms` (it imports `decorator_extensions`) started with
  `POST /api/extensions/<pkg>/start`. The process has no `TTS_CORE_URL`, and its
  log shows `Loading decorator Save Ogg` / `Loaded: decorator_save_flac, ...
  decorator_save_ogg_generator`, so `TTS_WEBUI_DECORATORS` (set by
  `extcatalog.DecoratorEnv` through `ExtraEnv`) is what `decorator_extensions`
  read; the route came up through the router (`GET /<pkg>/` 200) and went away
  on stop. mms runs in the main env here: no `.venvs/<pkg>` exists, so the
  venv-python path (a real native install) was not run.

Only in tests / not verified:
- Linux/macOS builds are not exercised here (venv path `bin/python`, process groups).
- `-race` not run (cgo unavailable).
- Graceful stop on Windows relies on `procs` (CTRL_BREAK, then job kill after 5s).
