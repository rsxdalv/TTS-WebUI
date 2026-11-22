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
| `GET /api/extcatalog/table?kind=` | | Markdown marker: the `Title / Description` table of every entry |
| `GET /api/extcatalog/picker?kind=&label=` | | Dropdown marker (`label`, `choices` = installable package names; interface sorted by name, decorators in file order) |
| `POST /api/extcatalog/parse` | `{"text"}` | `{"entries": [...], "preview": <Markdown marker>, "info": "Parsed entries: N \| Issues: ..."}` |
| `POST /api/extcatalog/add` | `{"entries": [...]}` | `{"status", "json"}`; appends to `tabs` of `extensions.external.json` (existing `package_name`s skipped) |
| `POST /api/extcatalog/install` | `{"package_name"}` | NDJSON stream; the package is looked up in the catalog files |
| `POST /api/extcatalog/install-entry` | `{"entry": {...}}` | NDJSON stream; the entry the caller gives is installed (its own requirements and proxy) |
| `POST /api/extcatalog/uninstall` | `{"package_name"}` | NDJSON stream |

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
  package name with optional version specifiers (`pkg[extra]>=1.0`) or an
  `https` / `git+https` URL. Refused: anything starting with `-` (pip takes
  options from the same place, so `--index-url`, `--extra-index-url` and `-e`
  are out), shell metacharacters (`; & | $ \``), control characters, local
  paths (`./x`, `/x`, `C:\x`, `..\x`), `ext::` (pip runs the command), and every
  URL scheme but `https` and `git+https` (so `http://`, `file://`,
  `git+file://`, `ssh://` and `git+ssh://` are out).
- A refused requirement is a line in the stream and `ok: false`; nothing runs.

This narrows what the tab can do compared to the Python code it replaces: the
old tab ran `pip_install_wrapper(requirements, name)` with the requirements
straight from the pasted text (`shell=True` for the venv path, so a pasted
`;` was a shell command). Go runs the requirements as arguments of `pip` /
`uv pip`, never through a shell, and refuses the ones that could turn pip into
something else. The by-name `install` endpoint is unchanged: it still only
installs what the catalog files list.

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

## Frozen copies (change both or neither)

- `torch.go` is `tts_webui/utils/get_torch_command.py` (torch 2.11.0, cu128).
- The merge and the Markdown table follow
  `tts_webui/extensions_loader/extensions_data_loader.py` and the old tab code.
  Checked on this repo: Go's `list` for both kinds equals Python's, key order
  included (66 interface entries, 1 decorator), and the table and picker choices
  are identical.
- `ordjson.go` keeps object key order so `extensions.external.json` can be
  rewritten without reshuffling a hand-edited file (indent 4, UTF-8 as is).
- `pyjson.go` is CPython's JSON scanner for the pasted text of the Extension
  Catalog tab: same error messages, same positions ("Expecting value: line 1
  column 1 (char 0)"), so the parse answer of the tab is the one it always
  showed. Checked against CPython 3.10 for 37 inputs
  (`TestPyParseMatchesCPython`).

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
  writer cannot write them back.
- No `True`/`False` line at the end of a pip stream (Python yielded the result
  flag into the console).
- pip runs as `python -m pip` in the main env instead of `pip` from `PATH`.
- The install of a native entry writes its log under the entry's `name`; the
  old Python tab wrote it under the same name.

## Stays in Python

The extension loader (`interface_extensions.py`, `decorator_extensions.py`,
`extensions_data_loader.py`: tab building, importing extensions, per-extension
management accordion, `setup_proxy_extension`), the JS "receive extension"
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
