# history: integration

Package `core/internal/serverpy/history`. The Gallery History tab
(`extensions/builtin/extension_gallery_history/main.py`) keeps its Gradio layout;
its logic (list, select, create, delete, copy, save to voices) is done by Go and
the Python handlers are calls through `tts_webui/core_bridge`
(`bridge.history.<op>(...)`, a table of verb and parameter names in
`core_bridge/history.py`; bound operations are used directly as Gradio handlers). This is the first "thin Gradio" tab.

## API (loopback, header `X-TTS-Core-Token`, otherwise 403)

Paths are relative to the project root. `/` and `\` are both accepted; answers
use `/`. Errors are `{"error": "..."}`: 400 bad path or body, 404 missing,
409 target exists, 403 token or non-loopback. POST bodies must be
`application/json` with no unknown fields. Python forwards its keyword
arguments by name, so the JSON field names are the Python parameter names.
A response is exactly the value the caller needs: a list, a marker, or an object
of markers (no wrapper object around a single value).

| Method and path | Body / query | Answer |
|---|---|---|
| `GET /api/history/collections` | | `["outputs","favorites","collections/<name>",...]` (names sorted) |
| `GET /api/history/items?collection=` | `outputs`, `favorites` or `collections/<name>` | `["outputs/<dir>/<dir>.png", ...]`: PNGs that have a `.wav` next to them, sorted |
| `GET /api/history/select?path=` | an item's `.png` or `.wav` | one marker per output control, see below |
| `POST /api/history/create-collection` | `{"name"}` | `{"collections": [...], "ui": <Button "Created">}`; 409 if it exists |
| `POST /api/history/delete` | `{"directory"}` (a generation directory) | `{"ok": true}` |
| `POST /api/history/copy` | `{"directory", "to"}` (`to` = a collection) | a marker; 409 if it exists |
| `POST /api/history/save-to-voices` | `{"npz"}` (an item's `.npz`) | the "Saved" Button marker; replaces `voices/<name>.npz` |

"Save to favorites" is `copy` with `to: "favorites"`.

### Gradio markers

Go cannot build a `gr.Button`; it sends a marker and the bridge
(`tts_webui/core_bridge/gradio_resolve.py`) builds the component:
`{"__gradio__": "Button", "props": {"value": "Saved"}}`. Only the components and
props listed in that file are accepted. `copy` answers a Button for favorites
and a Dropdown for any other collection.

`select` answers a dict that the handler maps onto its components by name:

| key | value |
|---|---|
| `bundle_name` | Markdown, the generation folder |
| `folder_root` | plain string, the generation folder (feeds delete/copy) |
| `audio`, `image` | Audio (value and label = the `.wav`), Image (the `.png`) |
| `json` | JSON, the item's `.json` |
| `npz` | Textbox, the `.npz` path (feeds save-to-voices) |
| `delete`, `save_to_voices`, `open_folder` | Button, `visible: true` |
| `save_to_favorites` | Button, hidden when the item is already in `favorites/` |

## Path safety

This replaces Python that ran `shutil.rmtree`/`copytree`/`copy` on strings from
hidden Textboxes. Go accepts an item or generation directory only if it
- starts with `outputs`, `favorites` or `collections/<name>` and is strictly
  below that root (a collection root itself cannot be deleted or copied);
- has no `..`, `.`, empty element, absolute path, drive letter or `:`, no
  control characters or `<>"|?*`, no leading or trailing dot or space, no
  Windows device name (`CON`, `NUL`, `COM1`...), and fewer than 200 characters per element;
- resolves, after following symlinks, to exactly the collection root plus the
  same elements: any link below a root, and a linked `collections/<name>`,
  is refused. `outputs/` and `favorites/` may themselves be a link the user made
  (for a bigger disk).

`copy` never overwrites, refuses a tree that contains a symlink or other
non-regular file, copies into a temp directory and renames, and creates
`outputs/` or `favorites/` if missing (server.py does the same at startup).
`save-to-voices` only takes a regular `.npz` inside a generation directory,
refuses a linked `voices/`, and writes through a temp file. Collection names go
through the same element rules. Writes are serialised; `delete` logs the path.

## What stays in Python

- All Gradio layout and events, and the global `audio_list_img` that maps a
  gallery click to an item. `update_history_tab` still builds the Gallery value
  from `bridge.history.items`.
- Media: Gradio gets relative paths on the same disk (forward slashes work on
  Windows); Go does not serve the files.
- "Open collection folder" / "Open folder" (`open_folder`, an OS action on the
  server machine).

## Differences from the Python code

- `items` returns only PNGs that have a sibling `.wav` (Python returned every PNG
  and crashed on select if the `.wav` was missing), sorted, hidden folders and
  symlinks skipped. `collections` lists only names that pass the element rules.
- Creating a collection that exists, and saving to a target that exists, are
  409 errors (Python raised too).
- The "Save to favorites" button is hidden for items already in `favorites/`.
  Python decided this from the tab's initial directory (always `outputs`), so it
  was always shown.
- `bundle_name` is a Markdown update (Python sent a Textbox update to a Markdown).

## Switch off

There is no Python fallback. Without `TTS_CORE_URL` (the launcher sets it only
when `TTS_GO_SERVERPY=1`) every bridge call raises `CoreBridgeError` ("the Go
core is not running"). That includes `get_collections()`, which runs when
`collections_directories_atom` is imported, so the Gallery History and History
extensions need the core to load. With the core set but unreachable the calls
raise as well.

## Wiring

`launcher/serverpy.go`: `mux.Handle(history.Prefix, history.New(...).Handler())`
on the loopback control API, token shared with appconfig. Python:
`tts_webui/core_bridge/` (`history.py` is this package's client),
`history_tab/{collections_directories_atom,save_to_favorites,delete_generation_cb}.py`
and the extension's `main.py`. `extension_history_tab` shares those helpers.

## Try it without the launcher

```
go build -o historyd.exe ./internal/serverpy/history/cmd/historyd
historyd.exe -root <dir with outputs/ favorites/ collections/> -addr 127.0.0.1:18081 -token dev
TTS_CORE_URL=http://127.0.0.1:18081 TTS_CORE_TOKEN=dev python <script that builds the tab>
```

Tests: `go test ./internal/serverpy/history/` and, from the repo root with the
project python, `python -m unittest tts_webui.core_bridge.test_core_bridge`.

## Found, not fixed (out of scope)

- The "Select directory" and "Save to collection" dropdowns do not refresh after
  "Create collection": `collections_directories_atom` is a `render=False`
  `gr.JSON`, so its `.change` never fires. A new collection shows up after a
  restart.
- `extension_history_tab` keeps its own `save_to_voices_cb` (plain
  `shutil.copy`) and its own select handlers; only the shared helpers moved.
- `tts_webui/history_tab/delete_generation.py` is unused and unchanged.
