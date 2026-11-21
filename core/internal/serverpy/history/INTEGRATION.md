# history: integration

Package `core/internal/serverpy/history`. The Gallery History tab
(`extensions/builtin/extension_gallery_history/main.py`) keeps its Gradio layout;
with `TTS_GO_SERVERPY=1` its file work (list, read, create, delete, copy) is done
by Go and the Python handlers are small HTTP calls
(`tts_webui/core_history_bridge`). This is the first "thin Gradio" tab.

## API (loopback, header `X-TTS-Core-Token`, otherwise 403)

Paths are relative to the project root. `/` and `\` are both accepted; answers
use `/` (the bridge turns them into `os.sep`). Errors are `{"error": "..."}`:
400 bad path or body, 404 missing, 409 target exists, 403 token or non-loopback.

| Method and path | Body / query | Answer |
|---|---|---|
| `GET /api/history/collections` | | `{"collections": ["outputs","favorites","collections/<name>",...]}` (same as `get_collections()`, names sorted) |
| `GET /api/history/items?collection=` | `outputs`, `favorites` or `collections/<name>` | `{"items": ["outputs/<dir>/<dir>.png", ...]}`: PNGs that have a `.wav` next to them, sorted |
| `GET /api/history/item?path=` | a `.wav` or `.png` path | `{"json": <the .json next to it>}` |
| `POST /api/history/create-collection` | `{"name"}` | `{"collections": [...]}`; 409 if it exists |
| `POST /api/history/delete` | `{"dir"}` (a generation directory) | `{"ok": true}` |
| `POST /api/history/copy` | `{"dir", "to"}` (`to` = a collection) | `{"ui": <marker>}` (the "Saved" update); 409 if it exists |

`ui` is a Gradio object marker, `{"__gradio__": "Button", "props": {"value": "Saved"}}`
(`Dropdown` for a collection other than favorites). The Python bridge
(`tts_webui/core_bridge/gradio_resolve.py`) turns it into the component; only
the components and props listed there are accepted.

POST bodies must be `application/json` with no unknown fields. "Save to
favorites" is `copy` with `to: "favorites"`.

## Path safety

This replaces Python that ran `shutil.rmtree`/`copytree` on a string from a
hidden Textbox. Go accepts a generation directory only if it
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
Collection names go through the same element rules. Writes are serialised.
`delete` logs the path at INFO.

## What stays in Python

- All Gradio layout, events and `gr.Gallery`/`gr.Audio`/`gr.Image` values.
- Media: Gradio still gets local relative paths on the same disk, as before;
  Go does not serve the files.
- "Open collection folder" / "Open folder" (`open_folder`, an OS action on the
  server machine) and "Save to voices" (copies an `.npz`): not moved.

## Differences from the Python code

- `items` returns only PNGs that have a sibling `.wav` (Python returned every PNG,
  and then crashed on select if the `.wav` was missing), sorted, hidden
  folders and symlinks skipped.
- `collections` lists only names that pass the element rules.
- Creating a collection that exists is a 409 error (Python raised `FileExistsError`).
- Saving to a target that exists is a 409 error (Python raised too).
- The "Saved" Button/Dropdown update comes from Go as a marker (see above).

## Switch off

`save_to_favorites` / `save_to_collection` (`bridge.history.copy`) have no
Python fallback: without `TTS_CORE_URL` (the launcher sets it only when
`TTS_GO_SERVERPY=1`) they raise `CoreBridgeError` ("the Go core is not running").
The other handlers (list, select, create, delete) still check
`core_history_bridge.enabled()` and run the original Python code when it is
off. With the core set but unreachable every bridge call raises; none falls
back to Python.

## Wiring

`launcher/serverpy.go`: `mux.Handle(history.Prefix, history.New(...).Handler())`
on the loopback control API, token shared with appconfig. Python edits:
`tts_webui/core_history_bridge/` (new), `history_tab/collections_directories_atom.py`,
`save_to_favorites.py`, `delete_generation_cb.py`, and `get_wav_files_img`,
`select_audio_history2`, `create_collection` in the extension's `main.py`.

## Try it without the launcher

```
go build -o historyd.exe ./internal/serverpy/history/cmd/historyd
historyd.exe -root <dir with outputs/ favorites/ collections/> -addr 127.0.0.1:18081 -token dev
TTS_CORE_URL=http://127.0.0.1:18081 TTS_CORE_TOKEN=dev python <script that builds the tab>
```

Tests: `go test ./internal/serverpy/history/` and, from the repo root with the
project python, `python -m unittest tts_webui.core_history_bridge.test_core_history_bridge`.

## Found, not fixed (out of scope)

- The "Select directory" and "Save to collection" dropdowns do not refresh after
  "Create collection": `collections_directories_atom` is a `render=False`
  `gr.JSON`, so its `.change` never fires. A new collection shows up after a
  restart. Same with and without the bridge.
- `get_collections()` in Python lists `collections/` under the project root but
  checks `isdir` against the current directory, so it assumes cwd is the root.
- With the bridge off the Python handlers still run the unchecked
  `rmtree`/`copytree` (by design: nothing changes when the port is off), and
  `tts_webui/history_tab/delete_generation.py` is unused and unchanged.
