# extcatalog: integration

Package `core/internal/serverpy/extcatalog`. Go side of three tabs: Extensions
List (`extension_list_tab`), Decorator Extensions List
(`extension_decorator_list_tab`) and Custom Extensions Installer
(`extensions/builtin/extension_custom_extensions_installer`). Python calls it
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
| `POST /api/extcatalog/install` | `{"package_name"}` | NDJSON stream |
| `POST /api/extcatalog/uninstall` | `{"package_name"}` | NDJSON stream |

The Markdown and Dropdown markers are built into components by the bridge where
they are called, so a handler placing them inside a `with gr.Column():` renders
them in that column.

### Streaming

`install` and `uninstall` answer `application/x-ndjson`: `{"line": "..."}` per
output line (stdout and stderr merged, `\r` and `\n` both end a line), then
`{"done": true, "ok": bool}`. The Python operation is a generator yielding the
lines so far joined with `<br />`, which is what the console `gr.HTML` showed
before. Failures are lines plus `ok: false`; they do not raise.

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

## Frozen copies (change both or neither)

- `torch.go` is `tts_webui/utils/get_torch_command.py` (torch 2.11.0, cu128).
- The merge and the Markdown table follow
  `tts_webui/extensions_loader/extensions_data_loader.py` and the old tab code.
  Checked on this repo: Go's `list` for both kinds equals Python's, key order
  included (66 interface entries, 1 decorator), and the table and picker choices
  are identical.
- `ordjson.go` keeps object key order so `extensions.external.json` can be
  rewritten without reshuffling a hand-edited file (indent 4, UTF-8 as is).

## Differences from the Python code

- `add` refuses to overwrite an `extensions.external.json` that is not a JSON
  object (Python warned and replaced it with an empty list).
- Go re-validates the entries it is sent; Python trusted its state.
- Installing takes a package name, so a pasted entry is added to the external
  list first and installed by name afterwards (the tab already did them in that
  order). Arbitrary requirements strings can no longer be run from the tab.
- The JSON parse error text is Go's, not Python's. No `True`/`False` line at the
  end of a pip stream (Python yielded the result flag into the console).
- pip runs as `python -m pip` in the main env instead of `pip` from `PATH`.

## Stays in Python

The extension loader (`interface_extensions.py`, `decorator_extensions.py`,
`extensions_data_loader.py`: tab building, importing extensions, per-extension
management accordion, `setup_proxy_extension`), the JS "receive extension"
button of the installer, and the Extension Catalog tab (not touched).

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
