# history: integration

Package `core/internal/serverpy/history`. Two tabs use it, both thin Gradio
over this package:

- the Gallery History tab (`extensions/builtin/extension_gallery_history`);
- the History tab (`extensions/builtin/extension_history_tab`): the Outputs >
  History tab and its Favorites and Collections siblings.

Their logic (list, select, create, delete, copy, save to voices) is done by Go
and the Python handlers are calls through `tts_webui/core_bridge`
(`bridge.history.<op>(...)`, a table of verb and parameter names in
`core_bridge/history.py`; bound operations are used directly as Gradio
handlers). The Gallery History tab was the first "thin Gradio" tab; the History
tab follows it.

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
| `GET /api/history/rows?directory=` | a collection, like `items` | the rows of the History table, see below |
| `GET /api/history/select?path=` | an item's `.png` or `.wav` | one marker per output control, see below |
| `POST /api/history/create-collection` | `{"name"}` | `{"collections": [...], "ui": <Button "Created">}`; 409 if it exists |
| `POST /api/history/delete` | `{"directory"}` (a generation directory) | `{"ok": true}` |
| `POST /api/history/copy` | `{"directory", "to"}` (`to` = a collection) | a marker; 409 if it exists |
| `POST /api/history/save-to-voices` | `{"npz"}` (an item's `.npz`) | the "Saved" Button marker; replaces `voices/<name>.npz` |

"Save to favorites" is `copy` with `to: "favorites"`.

### The rows of the History table

`rows` is the table of the History tab: one row per generation folder of the
collection, newest first. A row is a list of four strings:

| # | value |
|---|---|
| 0 | the timestamp of the folder name, ISO 8601 (`2023-06-02T10:54:24`); `1970-01-01T00:00:00` when the name has no parsable timestamp |
| 1 | the pretty name of the folder |
| 2 | how long ago it was made (`3 days ago`), or `Unknown` when the name has no timestamp |
| 3 | the `.wav` file of the folder, relative to the project root |

The date is an ISO string, which is exactly what Gradio sends to the browser
for a `datetime` object, so the Python side hands the rows to `gr.Dataframe`
as they are (`gr.Dataframe(value=bridge.history.rows(directory))`).

The rules of the name and the age are the ones of the old Python tab
(`tts_webui/history_tab/generate_pretty_name.py`, `generate_relative_date.py`
and `parse_time.py`, now removed): the timestamp is the first
`YYYY-MM-DD_HH-MM-SS` in the path, the name drops it and spells out the words
the generators put in it (`Bark history: none`, `Bark history: continued
(long)`), and the age counts days, then hours, then minutes, then seconds,
with the same "the seconds of a day, not the total" rule the Python had.

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
| `audio`, `image` | Audio (value and label = the `.wav`), Image (the `.png`, or no value when there is none) |
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

The hidden "Delete (API ONLY)" button of the History tab (the api the React UI
calls) went through `shutil.rmtree` on any path the caller supplied. It is now
`bridge.history.delete`, so it goes through the rules above. The React UI sends
`outputs/<dir>` or `favorites/<dir>`, which the rules accept.

## What stays in Python

- All Gradio layout and events, the global `audio_list_img` that maps a
  gallery click to an item. `update_history_tab` still builds the Gallery value
  from `bridge.history.items` and the Dataframe value from
  `bridge.history.rows`.
- The Dataframe of the History tab: Python hands the rows of Go to
  `gr.Dataframe` and maps the names of the answers of `select` onto its
  components.
- Media: Gradio gets relative paths on the same disk (forward slashes work on
  Windows); Go does not serve the files.
- "Open collection folder" / "Open folder" (`open_folder`, an OS action on the
  server machine). It stays in Python on purpose: it is not history logic but
  an OS call (`explorer`, `xdg-open`, `open`), it is shared with the gallery
  tab and with other extensions (`tts_webui/utils/OpenFolderButton.py`,
  `gr_reload_button.py`), and the React UI calls it by the name `open_folder`.
  Moving it would need an endpoint that runs a program with a path from the UI.

## Differences from the Python code

- `items` returns only PNGs that have a sibling `.wav` (Python returned every PNG
  and crashed on select if the `.wav` was missing), sorted, hidden folders and
  symlinks skipped. `collections` lists only names that pass the element rules.
- `rows` lists the folders the old `get_wav_files` listed, with the same names,
  the same ages and the same order. The differences:
  - the date is an ISO string instead of a `datetime` (the same bytes on the
    wire);
  - a folder whose name the path rules refuse (hidden, a link, a name with
    `:` or a device name) is not listed, because it could not be selected
    either; the old tab listed it;
  - folders with the same timestamp keep the order of their paths; the old tab
    kept the order of the directory scan, which is not the same every time;
  - a collection that does not exist is a 404; the old tab raised.
- The image is only shown when there is a `.png` next to the `.wav`. The old
  Gallery tab always had one, the old History tab checked with `os.path.exists`.
- `bundle_name` is a Markdown update (Python sent a Textbox update to a
  Markdown).
- A generation without a `.json` is a 404 on select; the old History tab
  swallowed the error and showed an empty JSON component.
- Creating a collection that exists, and saving to a target that exists, are
  409 errors (Python raised too).
- The "Save to favorites" button is hidden for items already in `favorites/`.
  Python decided this from the tab's initial directory (always `outputs`), so it
  was always shown.
- The api names of the tab are unchanged (`outputs_refresh_history`,
  `favorites_refresh_history`, `collections_refresh_history`, `save_to_voices`,
  `delete_generation`, `open_folder`). Gradio takes every api name once, so
  `save_to_voices` is now the name of the outputs tab's handler (it is the same
  operation with the same input as the favorites tab's one, which is registered
  as `save_to_voices_2`).

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
`history_tab/{collections_directories_atom,save_to_favorites,delete_generation_cb,delete_generation}.py`
and the two extensions' `main.py`. `extension_history_tab` shares the helpers of
`extension_gallery_history`.

## Try it without the launcher

```
go build -o historyd.exe ./internal/serverpy/history/cmd/historyd
historyd.exe -root <dir with outputs/ favorites/ collections/> -addr 127.0.0.1:18081 -token dev
TTS_CORE_URL=http://127.0.0.1:18081 TTS_CORE_TOKEN=dev python <script that builds the tab>
```

Tests: `go test ./internal/serverpy/history/` and, from the repo root with the
project python, `python -m unittest tts_webui.core_bridge.test_core_bridge`
(`HistoryTabTest` covers the rows, the delete and the api names of the tab).

## Found, not fixed (out of scope)

- The "Select directory" and "Save to collection" dropdowns do not refresh after
  "Create collection": `collections_directories_atom` is a `render=False`
  `gr.JSON`, so its `.change` never fires. A new collection shows up after a
  restart.
- Gradio takes an api name once, so the tabs of the same extension get
  `name_1`, `name_2`, ... for the second and third copy, and the numbering
  depends on the order the extensions are loaded. The api name
  `delete_generation` therefore belongs to the first delete handler that is
  registered (the visible delete button of the outputs tab, which takes a
  directory and a dropdown), and the hidden "Delete (API ONLY)" button of the
  favorites tab gets a `delete_generation_N` name. That is the same as before
  this port; the React UI has always called the first one.
