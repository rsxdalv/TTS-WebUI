# logview: integration

Package `core/internal/serverpy/logview`. The Log Viewer is now a builtin
extension, `extensions/builtin/extension_log_viewer/main.py` (a port of the
external `tts_webui_extension.log_viewer`, read from its repository, not run).
Python builds the Gradio layout; Go does every file operation on
`installer_scripts/logs` through `bridge.logs` (`tts_webui/core_bridge/logs.py`).
Without `TTS_CORE_URL` calls raise `CoreBridgeError` (no Python fallback).

## API (loopback, header `X-TTS-Core-Token`, otherwise 403)

`file` is always a bare file name in the logs directory, never a path.

| op | request | answer |
|---|---|---|
| `GET files` | | `[name, ...]`, newest first |
| `GET pip-logs?kind=install\|uninstall` | | `[{name, file}]`; `name` has the `pip-install-`/`pip-uninstall-` prefix and `.log` removed |
| `GET stats` | | Markdown: file count and total size (MB) |
| `GET summary?kind=` | | `install`: summary + status of the 10 newest installs; `uninstall`: count line |
| `GET info?file=` | | Markdown: name, size (KB), modified |
| `GET analyze?file=` | | Markdown analysis (status, counts, first unique lines) |
| `GET read?file=&search=&max_lines=` | | text; `max_lines` 1..100000 |
| `POST delete {file}` | | `{status, files}` |
| `POST delete-all {}` | | `{status, files}` |

Unknown query parameters, unknown or extra body fields and a wrong
Content-Type are 400. Messages the original showed in the text box stay in-band
(200): `File not found`, `Log file is empty`, `No matches found for 'x'`,
`[Showing last N of M lines]`, `Error reading file: ...`,
`Successfully deleted x`, `Deleted N log file(s)` (+ `Errors:` lines).

## Path safety

- A name must be one path element: no `/` or `\`, no `..`, no drive letter or
  UNC, no leading `.`, no trailing space, no control characters, none of
  `<>:"|?*`, no Windows reserved device names, at most 255 bytes, and it must end
  in `.log` (case-insensitive). Anything else is 400. Windows separators are
  rejected, not translated (the original never accepted paths either).
- The path is only ever `installer_scripts/logs/<name>`; there is no join of
  client text with anything else.
- Only regular files: the listing skips directories and symlinks; read, analyse,
  info and delete `Lstat` first and refuse a symlink or directory (400). Reads
  open the file and compare it with the `Lstat` result (`os.SameFile`), so a swap
  between check and open is refused. A symlink is never followed or deleted.
- Nothing outside the logs directory is touched; `delete-all` removes regular
  `*.log` files of that directory only. The directory is not created by Go (see
  differences).
- Tests: `TestValidName` (traversal, both separators, drive letters, UNC,
  reserved names, control characters, long names), `TestDelete` (a file next to
  the logs dir survives), `TestSymlinksAreNeverFollowed` (skipped when symlinks
  cannot be created), `TestHTTP` (traversal over the wire).

## Logging

INFO: `logview: deleted <name>` and `logview: deleted N log file(s)`. File
contents, search terms and listings are never logged. Errors other than 400 are
logged with the error text.

## Differences from the external extension

- Reads take at most the last 32 MiB of a file (the original read it whole) and
  say so in the text. `max_lines` is validated (1..100000); the UI still limits
  it to 100..10000. An empty Max Lines box falls back to 1000.
- The analysis lists the first unique lines in file order (the original listed an
  arbitrary subset of a Python set). Counts are the same.
- File name display strips only the leading `pip-install-`/`pip-uninstall-` and
  the trailing `.log` (the original `replace()` removed every occurrence).
- A missing logs directory is an empty list ("0 files", "0.00 MB"); the original
  created the directory on every call. Go creates it when an install writes a log.
- Symlinked or directory `*.log` entries are not listed and not deletable.
- The `.log` match is case-insensitive on every OS (glob was OS dependent).
- Invalid file names are an error toast (`CoreBridgeError`); they cannot come
  from the dropdown, which only offers listed files.
- The Gradio layout, labels and texts are unchanged; `Delete Selected` with no
  file shows "No file selected" in the status box like the original.

## Catalog change (extensions.json)

The existing entry (settings tab, "Log Viewer") had
`"package_name": "tts_webui_extension.log_viewer"`. Only that value changed, to
`"extensions.builtin.extension_log_viewer"`, the form the other builtin entries
use (the loader skips the pip install/update panel for names containing
`builtin`). `requirements`, the websites, description and the rest are
unchanged, as for the other builtin entries (e.g. `extension_gradio_settings`).
Consequences, for Rob to decide (Needs me card): the published external
extension is no longer listed from this file; a user's
`extensions.disabled`/`autostart` entry for the old name no longer applies; an
already installed external copy simply stays unused; and if the synced
`data/extensions-catalog` repository (or `extensions.external.json`) still lists
the old package name, that entry would show up as a second "Log Viewer".
Reverting is one line in `extensions.json` plus deleting the builtin folder.

## Wiring

`launcher/serverpy.go`: `mux.Handle(logview.Prefix, logview.New(...).Handler())`.
Dev server: `go run ./internal/serverpy/logview/cmd/logd -root <root> -token dev`,
then `TTS_CORE_URL=http://127.0.0.1:18085 TTS_CORE_TOKEN=dev`.

## Tests

Go: `go test ./internal/serverpy/logview/`. Python:
`python -m unittest tts_webui.core_bridge.test_core_bridge` (`LogViewerTest`:
named fields on the wire, the tab builds, handler defaults, wrong token, core
off).
