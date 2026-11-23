# extcatalog: integration

Package `core/internal/serverpy/extcatalog`. Go side of four tabs: Extensions
List (`extension_list_tab`), Decorator Extensions List
(`extension_decorator_list_tab`), Custom Extensions Installer
(`extensions/builtin/extension_custom_extensions_installer`) and Extension
Catalog (`extensions/builtin/extension_extension_catalog`). Python calls it
through `bridge.extcatalog.<op>(...)` (`tts_webui/core_bridge/extcatalog.py`); the
two list tabs share `tts_webui/extensions_loader/list_tab.py`.

## API (loopback, header `X-TTS-Core-Token`, otherwise 403)

Errors are `{"error": "..."}`: 400 bad input, 404 unknown package, 409 another
install is running. POST bodies are `application/json`, unknown fields refused.

| Method and path | Body / query | Answer |
|---|---|---|
| `GET /api/extcatalog/list?kind=` | `interface` or `decorator` | the merged entries (external > catalog > base, de-duplicated by `package_name`, `tabs` + `tabsInGroups`) |
| `GET /api/extcatalog/resolve?kind=&extension_class=` | `interface` or `decorator`; the class (`text-to-speech`, `outer`, ...) | `{"extensions": [{package_name, name, requirements, proxy, enabled, autostart}], "disabled_names": [...]}`: what Python loads, in order (see below) |
| `GET /api/extcatalog/table?kind=` | | Markdown marker: the `Title / Description` table of every entry |
| `GET /api/extcatalog/picker?kind=&label=` | | Dropdown marker (`label`, `choices` = installable package names; interface sorted by name, decorators in file order) |
| `POST /api/extcatalog/parse` | `{"text"}` | `{"entries": [...], "preview": <Markdown marker>, "info": "Parsed entries: N \| Issues: ..."}` |
| `POST /api/extcatalog/add` | `{"entries": [...]}` | `{"status", "json"}`; appends to `tabs` of `extensions.external.json` (existing `package_name`s skipped) |
| `POST /api/extcatalog/install` | `{"package_name"}` | NDJSON stream; the package is looked up in the catalog files |
| `POST /api/extcatalog/install-entry` | `{"entry": {...}}` | NDJSON stream; the entry the caller gives is installed (its own requirements and proxy) |
| `POST /api/extcatalog/uninstall` | `{"package_name"}` | NDJSON stream |
| `POST /api/extcatalog/check-dependencies` | `{"package_name"}` | NDJSON stream of `pip install --dry-run <requirements> gradio==5.49.1`; 400 for a native extension (it has no check) |
| `POST /api/extcatalog/latest-version` | `{"package_name"}` | a JSON string: the last word of pip's `Would install` line for `pip install --dry-run --no-deps <package> gradio==5.49.1`, or `Already up to date (sometimes incorrect)` |

The Markdown and Dropdown markers are built into components by the bridge where
they are called, so a handler placing them inside a `with gr.Column():` renders
them in that column.

### Streaming

`install`, `install-entry` and `uninstall` answer `application/x-ndjson`:
`{"line": "..."}` per output line (stdout and stderr merged, `\r` and `\n` both
end a line), then `{"done": true, "ok": bool}`. The Python operation is a
generator yielding the lines so far joined with `<br />`, which is what the
console `gr.HTML` showed before. Failures are lines plus `ok: false`; they do
not raise.

- Install looks the package up by name in the merged lists (interface first,
  then decorators), never takes a requirements string from the caller, and
  refuses built-ins. One install runs at a time (409 otherwise).
- Normal entries: `<main env python> -m pip install <requirements> gradio==5.49.1`
  (requirements split like `shlex.split`). Uninstall: `pip uninstall -y <package>`.
- `"proxy": "native"` entries: `.venvs/<package>` with `uv venv --allow-existing`,
  then `uv pip install --python <venv python>` for torch (from
  `installer_scripts/.gpu`, table in `torch.go`), `torchcodec`
  (`--index-url=https://download.pytorch.org/whl/cpu`), `xformers==0.0.35`, then the
  requirements plus the gradio compatibility pins. The package name must match
  `[A-Za-z0-9][A-Za-z0-9._-]*`. A failing step stops the rest.
- Logs: `installer_scripts/logs/{pip-install,pip-uninstall,uv-venv-install}-<name>.log`.
- The main env is the launcher's `micromamba -p <env> run` prefix (`Options.Run`),
  the same one the extension-process manager uses; `uv` is run the same way.

### One install implementation

Every install of the app goes through this package; Python has none left.
`interface_extensions.py` (the `[Available]` tab, Update, Uninstall, Check for
updates, Check Dependencies, and the venv install/update of native extensions)
and `generic_error_tab_advanced.py` (the Install button of a tab that failed to
load) call `bridge.extcatalog.install / uninstall / check_dependencies /
latest_version` with the package name; Go reads the requirements and the proxy
kind from the lists, so a button no longer carries a requirements string.

- `venv_setup_wrapper` and `get_torch_command.py` are deleted. The torch
  decision (`torch.go`, from `installer_scripts/.gpu`) is used only here, by the
  native install; no Python caller needs the command, so there is no op for it.
  `get_torchcodec_command` had no caller.
- `tts_webui/utils/pip_install.py` keeps `pip_install_wrapper` and
  `pip_uninstall_wrapper` (and `write_log`) for
  `installer_scripts/tools/extension_manager.py`, a separate CLI process without
  the core, which this step leaves alone. It is the only user left.
- The dry runs (`check-dependencies`, `latest-version`) take a package name from
  the lists like `install`; `latest-version` also checks that the name is a plain
  package name (`checkRequirement`) because it becomes a pip argument. Nothing
  here widens the validation of `install-entry`.
- A dependency check says what an install says (`Successfully installed ...`,
  `Please restart the webui ...`) because the old check did; it no longer
  overwrites `pip-install-<name>.log` (the old one did), and the update check
  writes no log (the old one wrote `pip-install-<package> (dry run, update check).log`).
- The `[Available]` and error-tab Install buttons run `install`, which uses the
  venv path for native entries. The old error tab always ran pip in the main env,
  also for a native extension.

### `install-entry`: the requirements are untrusted input

`install` takes a package name and reads the requirements from the catalog
files, so the caller cannot choose them. The Extension Catalog tab installs the
entries the user pasted, which is a different thing: the entry comes from the
browser (a pasted text, or a message from the catalog iframe), so its
requirements are untrusted.

`install-entry` therefore installs the entry it is given — its own
`requirements` and its own `proxy` — and validates the requirements hard
before anything runs (`entry.go`):

- The entry must be a complete extension entry (the same 12 keys the parse
  step checks), or the request is refused with 400.
- The requirements are split like `shlex.split` and each token must be a
  plain package name (PEP 508: `name`, `name[extra]`, `name>=1.0`,
  `name[extra]==1.0`) or an `https` / `git+https` URL (a host, a path, an
  optional `@ref`). The rule is a whitelist, so everything else is refused:
  anything starting with `-` (pip takes options from the same place, so
  `--index-url`, `--extra-index-url` and `-e` are out), shell metacharacters
  (`; & | $ \``), control characters, local paths (`./x`, `/x`, `C:\x`,
  `..\x`), `ext::` (pip runs the command), PEP 508 direct URLs
  (`name @ url`), and every URL scheme but `https` and `git+https` (so
  `http://`, `file://`, `git+file://`, `ssh://` and `git+ssh://` are out).
- A refused requirement is a line in the stream and `ok: false`; nothing runs.

This narrows what the tab can do compared to the Python code it replaces: the
old tab ran `pip_install_wrapper(requirements, name)` with the requirements
straight from the pasted text (`shell=True` for the venv path, so a pasted
`;` was a shell command). Go runs the requirements as arguments of `pip` /
`uv pip`, never through a shell, and refuses the ones that could turn pip into
something else. The by-name `install` endpoint is unchanged: it still only
installs what the catalog files list, and it does not validate them (they are
trusted data of the catalog, not input from the browser).

### The Extension Catalog tab

`extensions/builtin/extension_extension_catalog/main.py` is thin Gradio over
this API. It keeps the iframe, the `gr.Timer` and the `messaging_js` relay,
the hidden button and the wiring; the parse, the preview, the add and the
install are Go's.

The parsed entries live in a `gr.State` between the parse and the add/install
steps. The state travels through the browser, so it is client-controlled
either way: Go does not trust it and re-validates every entry it is sent
(the same checks as the parse step). Go takes the entries, not the text: the
text is parsed once (in Go), and the entries are what the add and the install
need.

## Which extensions load (`resolve`, `resolve.go`)

Python used to read `extensions.json`, the external file and the catalog clone,
merge them, apply `config["extensions"]` and order the result. Now Go answers
and Python imports and registers what it is told. `interface_extensions.
handle_extension_class(cls)` calls `resolve("interface", cls)` for each of the
ten tab classes of `all_tabs`; `decorator_extensions._load_decorators(cls)`
calls `resolve("decorator", cls)` for `outer` and `inner`.

- The list is the merged list (external > catalog > base) reduced to the
  entries whose `extension_type` is the kind and whose `extension_class` is the
  class, in merged order. Entries without a `package_name` cannot be imported
  and are not listed (logged); a missing `name` falls back to the
  `package_name`, a missing `requirements` is `""`.
- `enabled` is "not in `extensions.disabled`" (interface) or "not in
  `extensions.disabled_decorators`" (decorators); `autostart` is "in
  `extensions.autostart`" (interface only). Disabled entries stay in the list:
  the interface shows its `[Disabled] <name>` tab with the Enable button, the
  decorator loader prints that it skips it. `proxy` is `"native"` or `""`.
- `disabled_names` (decorators): the whole `disabled_decorators` list. It holds
  package names (already in `enabled`) and decorator function names
  (`decorator_save_ogg`), which only the importing side can match after the
  import; Python applies those.
- The lists come from `appconfig.Store.ExtensionLists` (`Options.Config`), read
  under the config mutex on every call, so what Go answers is what is in
  `config.json` now. A value of the wrong type is refused (500), never read as
  "nothing disabled".
- Python no longer holds `extension_list_json`, `disabled_extensions`,
  `autostart_extensions` or a `config["extensions"]` default for them. Every
  reader ran at build or import time (`_handle_package`, the `autostart` flag,
  the decorator module); they now take the answer of the call. That answer
  reflects `config.json` at the moment the tabs are built, so a toggle made
  earlier in the same run is visible to a tab built later (the UI still asks
  for a restart; tabs are built once, at startup).

### Decorators in extension processes

`decorator_extensions` is imported by extensions at import time
(`from tts_webui.extensions_loader.decorator_extensions import decorator_extension_outer`),
including those that run in their own process (native extensions). Those
processes get no `TTS_CORE_URL`/`TTS_CORE_TOKEN` (extprocs strips them: third-party
code must not hold the token). Go therefore puts the resolved decorators of both
classes into `TTS_WEBUI_DECORATORS` (`{"outer": ..., "inner": ...}`, computed by
`Store.DecoratorEnv` at each start through `extprocs.Options.ExtraEnv`, which wins
over the request's `env`). `_resolve` in `decorator_extensions.py` reads that
variable when it is set and asks Go otherwise. This keeps what those processes
load as it was; whether they should apply decorators at all is Rob's call.

## Frozen copies (change both or neither)

- `torch.go` is `tts_webui/utils/get_torch_command.py` (torch 2.11.0, cu128).
- The merge and the Markdown table follow
  `tts_webui/extensions_loader/extensions_data_loader.py` (reduced to
  `get_interface_extensions`, which `installer_scripts/tools/extension_manager.py`,
  a CLI without the core, still uses) and the old tab code.
  Checked on this repo: Go's `list` for both kinds equals Python's, key order
  included (66 interface entries, 1 decorator), and the table and picker choices
  are identical.
- `ordjson.go` keeps object key order so `extensions.external.json` can be
  rewritten without reshuffling a hand-edited file (indent 4, UTF-8 as is).
- `pyjson.go` is CPython's JSON scanner for the pasted text of the Extension
  Catalog tab: same error messages, same positions ("Expecting value: line 1
  column 1 (char 0)"), so the parse answer of the tab is the one it always
  showed. Checked against CPython 3.10 for 100 inputs
  (`testdata/pyjson_golden.jsonl`, written by
  `testdata/make_pyjson_golden.py`; `TestPyParseMatchesTheCPythonGolden`).
  The golden file covers the error messages, the floats (Python writes them
  with `repr()`: `1000.0`), the big integers (they keep their text) and the
  surrogate pairs (one character, like CPython's).

## Differences from the Python code

- `add` refuses to overwrite an `extensions.external.json` that is not a JSON
  object (Python warned and replaced it with an empty list).
- Go re-validates the entries it is sent; Python trusted its state.
- `install` takes a package name, so a pasted entry is added to the external
  list first and installed by name afterwards (the tab already did them in that
  order). The Extension Catalog tab uses `install-entry` instead, so a pasted
  entry can be installed without being in a catalog; its requirements are
  validated (see above).
- An entry whose `package_name` is already in the file is skipped, and a
  second entry with the same `package_name` in the *same* call is skipped too.
  The old Python tab added it twice (it did not update the set of known
  package names while it looped).
- The JSON parse error text is CPython's, not Go's (see the frozen copies).
  `NaN` and `Infinity`, which `json.loads` accepts, are refused: the file
  writer cannot write them back.- No `True`/`False` line at the end of a pip stream (Python yielded the result
  flag into the console).
- pip runs as `python -m pip` in the main env instead of `pip` from `PATH`.
- The install of a native entry writes its log under the entry's `name`; the
  old Python tab wrote it under the same name.
- Loading (`resolve`): an interface or decorator entry without `package_name` is
  skipped with a log line (the old loader raised `KeyError` while building the
  UI); a missing `name`/`requirements` no longer raises either.
- A disabled decorator package used to print `Loading decorator X....Skipping
  disabled decorator extension X` followed by `Failed to load: cannot unpack
  non-iterable NoneType object` (the skip returned `None`). It now prints only
  the skip line.
- The legacy `server.py` path without the core can no longer build the extension
  tabs or load decorators (every call raises `CoreBridgeError`), as for every
  other ported tab.

### `setup_proxy_extension.py` (not changed in this step)

What is left there is the Gradio side of a native extension (buttons, iframe,
state) plus process code that `core_ext_bridge` replaces at import (`subprocess`
is swapped for a shim that calls Go's `extprocs`): the venv python lookup, the
environment and `wait_for_server`. It also imports `gradio_proxy_tree`, which
`core_router_bridge` patches. Rewriting it to call Go directly belongs with the
removal of `server.py` and the proxy tree, so the dead spawn code goes in one
change with them.

## Stays in Python

The extension loader's interpreter work (`interface_extensions.py`,
`decorator_extensions.py`: building the tabs, importing extension modules,
matching decorator function names, the per-extension management accordion,
`setup_proxy_extension`), the JS "receive extension"
button of the installer, and in the Extension Catalog tab the iframe, the
`gr.Timer` with `messaging_js`, the hidden `receive_extension_button` and the
Gradio wiring.

`extensions.external.json` is written by Go only (`extcatalog.Add`): the tab
no longer reads or writes it.

## Switch off

No fallback: without `TTS_CORE_URL` these tabs raise `CoreBridgeError` when they
are built.

## Try it without the launcher

```
go build -o extcatalogd.exe ./internal/serverpy/extcatalog/cmd/extcatalogd
extcatalogd.exe -root <project root> -addr 127.0.0.1:18082 -token dev -python <python.exe>
TTS_CORE_URL=http://127.0.0.1:18082 TTS_CORE_TOKEN=dev python <script that builds the tab>
```

`-python` is the interpreter pip installs into. Tests: `go test
./internal/serverpy/extcatalog/` (a fake python/uv stands in for the env) and
`python -m unittest tts_webui.core_bridge.test_core_bridge`.
