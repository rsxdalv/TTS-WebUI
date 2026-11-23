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

## Operations (single-setting changes)

Python no longer mutates a shared dict and PUTs it. Each operation is a POST
under `/api/config/` (same token, JSON object body, unknown or missing fields
are 400) and runs through `Store.update`: under the Store mutex it reads the
file (default created when missing), changes one thing, validates, writes (only
when something changed) and logs launch-time changes. It is the only way
besides `Save`, so concurrent toggles cannot lose each other (tests: 40
goroutines).

| op | body | answer |
|---|---|---|
| `extension-enable` / `extension-disable` | `package_name` | `{changed, disabled, info}` |
| `extension-toggle` | `package_name` | `{changed: true, disabled, info}`; Go decides from the file |
| `extension-autostart` | `package_name`, `enabled` (bool) | `{changed, enabled}` |
| `set-value` | `namespace`, `key`, `value` (any JSON) | `{changed}` |
| `replace-section` | `namespace`, `values` (object) | `{changed}` |

- `extensions.disabled` / `extensions.autostart`: add appends only when absent,
  remove drops every occurrence; both are idempotent. A missing list or
  `extensions` object is created; one of the wrong type is refused (400), never
  overwritten. `package_name` must match `[A-Za-z0-9][A-Za-z0-9_.-]{0,199}`;
  `namespace` and `key` are non-empty strings without control characters.
- `info` is the toast text; the Python handler passes it to `gr.Info` (a plain
  field, no marker needed). Wording is the old Python wording.
- Values are never logged (names of launch-time keys only), as for `PUT`.
- `server_port` must stay an integer for every operation (same check as `PUT`).

Python: `bridge.config.<op>` (`tts_webui/core_bridge/config.py`). Call sites:
`interface_extensions.py` (`enable_extension`, `disable_extension`,
`toggle_extension_state`, `toggle_autostart`), `config_utils.set_config_value`,
`save_config_gradio.save_config_gradio`. Without `TTS_CORE_URL` they raise
`CoreBridgeError` (no Python fallback).

### Stale in-memory state (decision)

After a change the process's own `config` keeps the startup values; the UI
already asks for a restart. The extension lists are no longer held in Python at
all: `disabled_extensions`, `autostart_extensions`, `extension_list_json` and the
`config["extensions"]` defaults were removed, and the loader asks Go
(`extcatalog` `resolve`, which reads them through `Store.ExtensionLists`).
Two places would have shown the difference and are handled:
- `toggle_extension_state` used to decide from the in-memory list; it now asks
  Go to toggle, so Disable/Enable alternate correctly on repeated clicks and
  the button label comes from Go's answer.
- `set_config_value` / `save_config_gradio` mirror the written value into the
  in-memory `config` after Go succeeded, so `get_config_value` stays coherent.
The extension lists are not mirrored.

`Store.ExtensionLists()` (`lists.go`) is the read side for the loader: the
`disabled`, `autostart` and `disabled_decorators` lists, read under the same
mutex as every write. A missing section or key is an empty list; a value of
another type is an error; non-string entries are dropped (they match no package
name). Reading creates the default file when it is missing, like `GET`.

### Stays in Python

- `installer_scripts/tools/extension_manager.py` (a separate CLI process
  without the core) still calls `_save_config` and writes the file itself.
- `core_config_bridge` (whole-document GET/PUT) stays for loading the config at
  startup and for any caller of `_save_config` left (only that CLI).
- `get_config_value` reads the in-memory dict.

Dev server: `go run ./internal/serverpy/appconfig/cmd/configd -root <dir> -token dev`.

## Other writers

- Python callers `save_config_gradio`, `set_config_value` and
  `interface_extensions` now use the operations above, not `_save_config`.
- `installer_scripts/tools/extension_manager.py` is a separate CLI process that
  calls `_save_config` without `TTS_CORE_URL`: it still writes the file itself.
- React (`react-ui/src/pages/api`) does not write `config.json`.

## Known limit

`core_router_bridge` rewrites `server_port`/`server_name`/`inbrowser` in the
in-memory config (the private Gradio port). The operations above no longer send
that dict back, so this leak is gone for the three ported call sites; only the
`extension_manager.py` CLI (own process, own config) is unaffected.
