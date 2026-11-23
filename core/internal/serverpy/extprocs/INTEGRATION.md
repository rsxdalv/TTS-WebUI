# extprocs: wiring (stage 4)

Go runs the proxy-extension processes; `setup_proxy_extension.py` asks Go
through `bridge.extensions` (`tts_webui/core_bridge/extensions.py`) and no
longer spawns anything (step 27; before, `tts_webui/core_ext_bridge` patched
`subprocess` in that module at import, and the file kept the dead spawn code).

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
3. **Entry script**: run `server_core.py` (stage 3) instead of `server.py`.

## Behaviour notes

- API (loopback + `X-TTS-Core-Token`, POST needs `application/json`; else 403/415):
  `POST /api/extensions/{pkg}/start` `{"port","openai_proxy_host"}` -> `{"port","pid"[,"already_running"]}`
  (Python sends only `openai_proxy_host`; Go picks the port when none is asked for);
  `POST .../stop` (404 if never started); `GET /api/extensions`;
  extra: `GET /api/extensions/{pkg}/logs?tail=N` -> `{"lines":[...]}`.
- No `env` in the start body (step 27): extension processes get `BaseEnv`
  (the launcher's merged environment: launcher env, `.env`, `.env.user`) plus
  `ExtraEnv`, so Python has no variables to hand over. `TTS_CORE_TOKEN` and
  `TTS_CORE_URL` are never passed to extension processes.
- `Options.ExtraEnv` (optional) returns `KEY=VALUE` variables Go computes for
  each extension at start; they win over `BaseEnv`. The
  launcher uses it for `TTS_WEBUI_DECORATORS` (the resolved decorators; see
  extcatalog), because extension processes have no control token. If it fails,
  the extension is not started.
- Go registers `/<pkg>` itself once the port accepts TCP and only then reports
  `ready` (a restart after Stop gets its route back). Python does not add the
  route.
- Python side (`bridge.extensions`): `start(package, openai_proxy_host)` returns
  when the process is spawned; `wait_ready(package)` polls `GET /api/extensions`
  until `ready` and raises `CoreBridgeError` with Go's reason on `failed` or
  `exited` (or after 130 s); `stop(package)` treats 404 as already stopped. The
  tab stops the process, shows the reason and re-enables Start when the start fails (the old code
  built a `gr.Error` without raising it and showed the iframe anyway).
- ReadyTimeout (default 120s): state becomes `failed` but the process keeps
  running and probing; if it comes up later it turns `ready` and is routed.

## Verified for real vs only in tests

Real:
- `go test ./internal/serverpy/extprocs/...` and `go vet` pass on Windows
  (go1.27.4): fake python built from `testdata/fakepy` with real processes, ports, job-object kill.
- `TestRealExtension` (opt-in: `EXTPROCS_REAL_PYTHON`, `EXTPROCS_REAL_ROOT`) started the
  installed `tts_webui_extension.log_viewer` through the Manager using the main env's
  `python.exe`: reached `ready`, route registered, `GET /<pkg>/` returned 200, `Stop`
  removed the route. No `.venvs/<pkg>` extension is installed, so the venv-python path was
  only tested with the fake.
- Python: `python -m unittest tts_webui.core_bridge.test_core_bridge`
  (`ExtensionsBridgeTest`, `SetupProxyExtensionTest`: start, wait, stop, failure,
  and that `setup_proxy_extension.py` holds no process code).

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
