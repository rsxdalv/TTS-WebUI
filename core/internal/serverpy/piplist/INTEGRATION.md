# piplist: integration

Package `core/internal/serverpy/piplist`. Settings > Installed Packages
(`extensions/builtin/extension_pip_list/main.py`) lists the main env's packages
through `bridge.pip.list()` (`tts_webui/core_bridge/pip.py`). Without
`TTS_CORE_URL` the call raises `CoreBridgeError` (no Python fallback).

## API (loopback, header `X-TTS-Core-Token`, otherwise 403)

`GET /api/pip/list` -> `[{"name": str, "version": str}, ...]` (the shape of the
hidden `get_pip_list` API button, unchanged).

## How the list is made

Go runs `<Run prefix> python -m pip list --format=json --disable-pip-version-check`
in the project root, with the launcher's environment (`Run` is the same
`micromamba -p <env> run` prefix `extcatalog` uses; `Options.Python` replaces it
for the `pipd` dev server). Why pip and not reading `site-packages` from Go:
pip answers from the interpreter's own `sys.path` (editable installs, `.pth`,
egg-info, user site), so Go does not have to guess the layout per platform; it
is the same `importlib.metadata` data Python listed before. Cost: starting
Python (about a second); nothing is cached, "Refresh" runs it again. One pip at
a time (mutex), 2 minute timeout, 16 MiB output cap.

## Decisions

- **Order:** case-insensitive by name, then exact name, then version. Python's
  `importlib.metadata.distributions()` order was arbitrary; the Markdown lines
  (`name==version`, blank line between) are rendered by the unchanged
  `render_pip_list`.
- **Failing pip:** HTTP 502 with `{"error": "pip list failed: <exit error>: <first 3
  stderr lines, 300 chars>"}` (also INFO-logged as `piplist: ...`), unreadable
  output and timeouts likewise. Python raises `CoreBridgeError`, which Gradio shows
  as an error toast; the Markdown keeps its previous text. Stderr text is the only
  thing from pip that is logged; the package list is not.
- Differences from the old output: pip omits nothing but may list `pip`,
  `setuptools` etc. like the old code did; distributions with an empty name are
  dropped.

## Wiring

`launcher/serverpy.go`: `mux.Handle(piplist.Path, piplist.New(...).Handler())`
with `Run: prefix`, `Env: baseEnv`.

## Dev server

`go run ./internal/serverpy/piplist/cmd/pipd -root <root> -token dev -python <python.exe>`,
then `TTS_CORE_URL=http://127.0.0.1:18084 TTS_CORE_TOKEN=dev`.

## Tests

Go: `go test ./internal/serverpy/piplist/` (fake pip via the test binary: ordering,
failing pip, garbage output, timeout, missing program, HTTP auth/method/path).
Python: `python -m unittest tts_webui.core_bridge.test_core_bridge` (`PipTest`:
the tab's `get_pip_list`/`refresh_pip_list`, wrong token, core off).
