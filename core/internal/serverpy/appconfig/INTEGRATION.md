# appconfig: integration

Package `core/internal/serverpy/appconfig`. With `TTS_GO_SERVERPY=1` Go is the
only process that reads or writes `config.json`; Python goes through
`tts_webui/core_config_bridge`. Switch off: nothing changes.

## API (loopback, header `X-TTS-Core-Token`, otherwise 403)

- `GET /api/config`: the file's bytes; a missing file is created from the
  defaults of `load_config.py` first.
- `PUT /api/config` (`Content-Type: application/json`): replaces the document.
  400 unless it is a JSON object and `gradio_interface_options.server_port`, when
  present, is an integer.

## Bytes

The file is written as `json.dump(data, f, indent=2)`: key order kept, numbers
keep their literal, ensure_ascii escapes (including `\u007f` and surrogate
pairs), no HTML escaping, no trailing newline. Temp file + rename under a mutex.
`testdata/golden.json` comes from `testdata/gen_golden.py` (run from the repo
root with the project python); the Go tests compare byte for byte. Float
literals are not re-normalised (`1e-7` stays); the Python client sends what
Python's `json.dumps` produced, so this only matters for hand-edited files.

## Logging

INFO line when `gradio_interface_options` `server_port`, `server_name`, `auth`,
`share` or `ssl_*` changed: they take effect after a restart. Names only, never
values. A warning if `appenv.ParseConfig` (the parsing the launcher uses, split
out of `LoadConfig`) says server.py would fail on the new document.

## Wiring

`launcher/serverpy.go`: `mux.Handle(appconfig.Path, appconfig.New(...).Handler())`.
`server_core.py`: `install_config()` runs **first**, because `core_router_bridge`
imports `tts_webui.config.config`, which loads the config at import. The bridge
replaces `load_config.load_config`, `_save_config._save_config` and
`_save_config._save_json` (for `config.json`) before anything binds them.

## Other writers

- Python callers of `_save_config` (`save_config_gradio`, `set_config_value`,
  `interface_extensions`) all use that function: covered.
- `installer_scripts/tools/extension_manager.py` is a separate CLI process that
  calls `_save_config` without `TTS_CORE_URL`: it still writes the file itself.
- React (`react-ui/src/pages/api`) does not write `config.json`.

## Known limit

`core_router_bridge` rewrites `server_port`/`server_name`/`inbrowser` in the
in-memory config (the private Gradio port). A later Settings save persists that
dict, as it did before this package; fixing it is outside this unit.
